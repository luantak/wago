//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math/bits"
	"os"
)

var integerPacketVectorEnabled = os.Getenv("WAGO_AMD64_INTEGER_PACKET_VECTOR") == "1"
var integerPacketAssertFast bool

// Pack independent expression lanes inside a source iteration, rather than
// assuming consecutive iterations have a fixed address stride. Every failed
// range guard leaves the entire current packet for the unchanged scalar body.
// One final source iteration always restores all temporary local versions.
func (f *fn) emitIntegerPacket(source *wasm.Reader) {
	if !f.opt(optIntegerPacketVector) || f.integerMemory != nil || f.pureReduce != nil || f.interruptible || f.moduleEH || f.localBase != 0 || len(f.customInstructions) != 0 || f.intervalActive != 0 || f.pinRelinquished || f.threadedMemory0 || f.gcFrameRoots != nil || f.depth() != 0 || f.m.MemCount() == 0 || f.memoryAddr64(0) {
		return
	}
	features := shared.AMD64AVX | shared.AMD64SSE41
	if !f.sc.amd64Features.Has(features) {
		return
	}
	p, ok := f.inspectIntegerPacket(source)
	if !ok {
		return
	}
	block := f.pinned.union(f.pinnedLocalMask).union(f.reserved)
	var gp [6]Reg
	needGP := 5
	if p.program {
		needGP = 6
	}
	ng := 0
	for _, r := range gpAlloc {
		if !block.has(r) && f.regUser[r] == nil {
			gp[ng] = r
			ng++
			if ng == needGP {
				break
			}
		}
	}
	if ng != needGP {
		return
	}
	fblock := f.fpinned.union(f.fpinnedLocalMask).union(f.fconstMask()).union(f.v128ConstMask())
	var fp [16]Reg
	nf := 0
	for r := Reg(0); r < 16; r++ {
		if !fblock.has(r) && f.fregUser[r] == nil {
			fp[nf] = r
			nf++
		}
	}
	if nf < p.n+1 {
		return
	}
	var cached [16][8]Reg
	for i := range cached {
		for c := range cached[i] {
			cached[i][c] = regNone
		}
	}
	fpCount := p.n + 1
	if f.opt(optPacketInputHoist) {
		chunks := p.lanes / 4
		for i, n := range p.nodes[:p.n] {
			if n.op == 0xfd && fpCount+chunks-1 <= nf {
				cached[i][0] = fp[i]
				for c := 1; c < chunks; c++ {
					cached[i][c] = fp[fpCount]
					fpCount++
				}
			}
		}
	}
	vectorReg := func(node, chunk int) Reg {
		if cached[node][chunk] != regNone {
			return cached[node][chunk]
		}
		return fp[node]
	}
	oldPinned, oldFPinned := f.pinned, f.fpinned
	for _, r := range gp[:needGP] {
		f.pinned = f.pinned.add(r)
	}
	for _, r := range fp[:fpCount] {
		f.fpinned = f.fpinned.add(r)
	}
	defer func() { f.pinned, f.fpinned = oldPinned, oldFPinned }()
	f.cpuHas(features)
	f.pureReduceFrameUsed = true
	counter, index, acc, address, tmp := gp[0], gp[1], gp[2], gp[3], gp[4]
	sum := acc
	if p.program {
		sum = gp[5]
	}
	read := func(local int, dst Reg) {
		if reg, _, pin := f.pinReg(local); pin {
			f.recoverLocal(local)
			f.moveInt(dst, reg, mtI32)
		} else if f.locals[local].state == lsConstZero {
			f.a.XorSelf32(dst)
		} else {
			f.loadFrameInt(dst, f.localAddr(local), mtI32)
		}
	}
	fillLanes := func(dst Reg, node, chunk int) {
		for lane := 0; lane < 4; lane++ {
			input := p.laneInputs[node][p.laneOrder[chunk*4+lane]]
			if input.op == 0x41 {
				f.a.MovImm32(address, int32(input.v))
			} else {
				read(input.a, address)
			}
			if lane == 0 {
				f.a.MovGprToXmm(dst, address, false)
			} else {
				f.a.Pinsrd(dst, address, byte(lane))
			}
		}
	}
	// All recovery precedes the first conditional edge. A zero/one-count path
	// must not skip a reload the compiler has already recorded as complete.
	for i, n := range p.nodes[:p.n] {
		switch n.op {
		case 0x20:
			read(n.a, address)
			f.a.MovGprToXmm(fp[i], address, false)
			f.a.Pshufd(fp[i], fp[i], 0)
		case 0x41:
			f.a.MovImm32(address, int32(n.v))
			f.a.MovGprToXmm(fp[i], address, false)
			f.a.Pshufd(fp[i], fp[i], 0)
		case 0x28:
			if n.a >= 0 {
				read(n.a, address)
			}
		case 0xfd:
			if cached[i][0] != regNone {
				for chunk := 0; chunk < p.lanes/4; chunk++ {
					fillLanes(cached[i][chunk], i, chunk)
					f.stats.peep("integer-packet-input-hoist")
				}
				continue
			}
			for _, input := range p.laneInputs[i][:p.lanes] {
				if input.op == 0x20 {
					read(input.a, address)
				}
			}
		}
	}
	for _, step := range p.post[:p.postN] {
		if step.operand.op == 0x20 {
			read(step.operand.a, tmp)
		}
	}
	if p.stride.op == 0x20 {
		read(p.stride.a, tmp)
	}
	read(p.counter, counter)
	read(p.index, index)
	read(p.acc, acc)
	if integerPacketAssertFast {
		f.a.AluRI(cmpDigit, counter, 1, false)
		valid := f.a.JccPlaceholder(condA)
		f.trapAlways(trapUnreachable)
		f.a.PatchRel32(valid, f.a.Len())
	}
	var exits [17]int
	ne := 0
	loop := f.a.Len()
	f.a.AluRI(cmpDigit, counter, 1, false)
	exits[ne] = f.a.JccPlaceholder(condBE)
	ne++
	loadAddress := func(n integerPacketNode) {
		if n.a >= 0 {
			read(n.a, address)
		} else {
			f.a.XorSelf32(address)
		}
		f.a.Add32(address, index)
		if n.v != 0 {
			f.a.AluRI(0, address, int32(n.v), false)
		}
	}
	// Validate every complete stream before updating the accumulator. This
	// includes the last lane and any i32 address wrap; no speculative overread.
	for _, n := range p.nodes[:p.n] {
		if n.op == 0x28 {
			loadAddress(n)
			f.a.LeaDisp(tmp, address, int32(p.lanes*4))
			f.a.AluRM(cmpRMcode, tmp, RBX, -bdCurBytes, true)
			exits[ne] = f.a.JccPlaceholder(condA)
			ne++
		}
	}
	if p.program {
		f.a.XorSelf32(sum)
	}
	for chunk := 0; chunk < p.lanes/4; chunk++ {
		for i, n := range p.nodes[:p.n] {
			dst := fp[i]
			a, b := regNone, regNone
			if n.op >= 0x6a && n.op <= 0x76 {
				a, b = vectorReg(n.a, chunk), vectorReg(n.b, chunk)
			}
			switch n.op {
			case 0xfd:
				if cached[i][chunk] == regNone {
					fillLanes(dst, i, chunk)
				}
			case 0x28:
				loadAddress(n)
				f.a.VMovdquLoadIdx(dst, RBX, address, int32(chunk*16))
			case 0x6a:
				f.a.VPaddd(dst, a, b)
			case 0x6b:
				f.a.VPsubd(dst, a, b)
			case 0x6c:
				f.a.VPmulld(dst, a, b)
			case 0x71:
				f.a.VPand(dst, a, b)
			case 0x72:
				f.a.VPor(dst, a, b)
			case 0x73:
				f.a.VPxor(dst, a, b)
			case 0x74:
				f.a.VPslldImm(dst, a, byte(p.nodes[n.b].v&31))
			case 0x75:
				f.a.VPsradImm(dst, a, byte(p.nodes[n.b].v&31))
			case 0x76:
				f.a.VPsrldImm(dst, a, byte(p.nodes[n.b].v&31))
			}
		}
		root, shuffle := fp[p.root], fp[p.n]
		f.a.Pshufd(shuffle, root, 0x4e)
		f.a.VPaddd(root, root, shuffle)
		f.a.Pshufd(shuffle, root, 0xb1)
		f.a.VPaddd(root, root, shuffle)
		f.a.MovXmmToGpr(tmp, root, false)
		f.a.Add32(sum, tmp)
	}
	if p.program {
		for _, step := range p.post[:p.postN] {
			v := step.operand
			if v.op == 0x41 {
				f.a.MovImm32(tmp, int32(v.v))
			} else {
				switch v.a {
				case p.acc:
					f.moveInt(tmp, acc, mtI32)
				case p.index:
					f.moveInt(tmp, index, mtI32)
				case p.counter:
					f.moveInt(tmp, counter, mtI32)
				default:
					read(v.a, tmp)
				}
			}
			switch step.op {
			case 0x6a:
				f.a.Add32(sum, tmp)
			case 0x6b:
				if step.reverse {
					f.a.Sub32(tmp, sum)
					f.moveInt(sum, tmp, mtI32)
				} else {
					f.a.Sub32(sum, tmp)
				}
			case 0x6c:
				f.a.IMul(sum, tmp, false)
			case 0x71:
				f.a.AluRR(0x21, sum, tmp, false)
			case 0x72:
				f.a.AluRR(0x09, sum, tmp, false)
			case 0x73:
				f.a.AluRR(0x31, sum, tmp, false)
			case 0x74:
				f.a.ShiftImm(4, sum, byte(v.v&31), false)
			case 0x75:
				f.a.ShiftImm(7, sum, byte(v.v&31), false)
			case 0x76:
				f.a.ShiftImm(5, sum, byte(v.v&31), false)
			case 0x6d:
				k := byte(bits.TrailingZeros32(v.v))
				if k != 0 {
					f.moveInt(tmp, sum, mtI32)
					f.a.ShiftImm(7, tmp, 31, false)
					f.a.AluRI(4, tmp, int32(v.v-1), false)
					f.a.Add32(sum, tmp)
					f.a.ShiftImm(7, sum, k, false)
				}
			case 0x6e:
				f.a.ShiftImm(5, sum, byte(bits.TrailingZeros32(v.v)), false)
			}
		}
		f.moveInt(acc, sum, mtI32)
	}
	if p.stride.op == 0x20 {
		read(p.stride.a, tmp)
	} else {
		f.a.MovImm32(tmp, int32(p.stride.v))
	}
	f.a.Add32(index, tmp)
	f.a.AluRI(5, counter, 1, false)
	f.a.PatchRel32(f.a.JmpPlaceholder(), loop)
	if integerPacketAssertFast {
		for _, site := range exits[1:ne] {
			f.a.PatchRel32(site, f.a.Len())
		}
		f.trapAlways(trapUnreachable)
	}
	f.a.PatchRel32(exits[0], f.a.Len())
	if !integerPacketAssertFast {
		for _, site := range exits[1:ne] {
			f.a.PatchRel32(site, f.a.Len())
		}
	}
	for _, state := range []struct {
		local int
		reg   Reg
	}{{p.counter, counter}, {p.index, index}, {p.acc, acc}} {
		if reg, _, pin := f.pinReg(state.local); pin {
			f.moveInt(reg, state.reg, mtI32)
			f.locals[state.local].state = lsStackReg
		} else {
			f.locals[state.local].state = lsMem
		}
		f.storeFrameInt(f.localAddr(state.local), state.reg, mtI32)
	}
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.stats.peep("integer-packet-vector")
}
