//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math/bits"
	"os"
)

var integerMemoryVectorEnabled = os.Getenv("WAGO_AMD64_INTEGER_MEMORY_VECTOR") == "1"
var integerCountedVectorEnabled = os.Getenv("WAGO_AMD64_INTEGER_COUNTED_VECTOR") == "1"
var integerPairedLoadsEnabled = os.Getenv("WAGO_AMD64_INTEGER_PAIRED_LOADS") == "1"
var integerStoreVectorEnabled = os.Getenv("WAGO_AMD64_INTEGER_STORE_VECTOR") == "1"
var integerMapVectorEnabled = os.Getenv("WAGO_AMD64_INTEGER_MAP_VECTOR") == "1"
var integerMapGroupGuardEnabled = os.Getenv("WAGO_AMD64_INTEGER_MAP_GROUP_GUARD") == "1"
var integerRegionVectorEnabled = os.Getenv("WAGO_AMD64_INTEGER_REGION_VECTOR") == "1"

// Tests can distinguish a taken vector path from a silently successful fallback.
var integerMemoryAssertFast bool

// All guards precede guest loads. Failure enters the unchanged checked scalar
// body. Successful groups preserve modulo-i32 arithmetic and all local homes.
func (f *fn) emitIntegerMemoryLoop(r *wasm.Reader) {
	// The loop proof rejects calls and nested control. At an inactive region
	// boundary, existing pre-guard recovery and two-home writes preserve the
	// caller's local state even when the containing function calls elsewhere.
	regionOK := f.opt(optIntegerRegionVector) && f.intervalActive == 0
	if (!f.opt(optIntegerMemoryVector) && !f.opt(optIntegerCountedVector) && !f.opt(optIntegerStoreVector) && !f.opt(optIntegerMapVector)) || f.integerMemory != nil || f.pureReduce != nil || (!regionOK && (f.usesCalls || len(f.intervalReg) != 0)) || f.interruptible || f.moduleEH || f.localBase != 0 || len(f.customInstructions) != 0 || f.depth() != 0 || f.pinRelinquished || f.threadedMemory0 || f.gcFrameRoots != nil {
		return
	}
	features := shared.AMD64AVX | shared.AMD64SSE41
	if !f.sc.amd64Features.Has(features) {
		return
	}
	wide := f.sc.amd64Features.Has(shared.AMD64AVX2)
	lanes := int32(4)
	if wide {
		features |= shared.AMD64AVX2
		lanes = 8
	}
	var p integerMemoryPlan
	ok := false
	if f.opt(optIntegerMemoryVector) && f.m.MemCount() != 0 && !f.memoryAddr64(0) {
		p, ok = f.inspectIntegerMemoryLoop(r)
	}
	if !ok && wide && f.opt(optIntegerCountedVector) {
		p, ok = f.inspectIntegerLoop(r, true)
		ok = ok && p.loadN == 0
	}
	if !ok && wide && f.opt(optIntegerStoreVector) && f.m.MemCount() != 0 && !f.memoryAddr64(0) {
		p, ok = f.inspectIntegerLoopWithStores(r, true, true)
	}
	if !ok && wide && f.opt(optIntegerMapVector) && f.m.MemCount() != 0 && !f.memoryAddr64(0) {
		p, ok = f.inspectIntegerMapLoop(r, true, true, true)
		ok = ok && p.loadN > 0
	}
	if !ok {
		return
	}
	for _, load := range p.loads[:p.loadN] {
		if load.stride != 0 && load.stride != 4 && load.stride != 8 && !(p.mapLoads && load.stride == 16) {
			return
		}
	}
	q := pureReducePlan{nodes: p.nodes, values: p.values, written: p.written, n: p.n, locals: p.locals, counter: p.induction, induction: p.induction, step: p.step, reductions: p.reductions, nReductions: p.reductionN}
	var storePartner [8]int
	storeQuads, quadsOK := integerMemoryQuads(p.stores[:p.storeN])
	if !quadsOK {
		return
	}
	for i := range storePartner {
		storePartner[i] = -1
	}
	storeDelta := func(a, b integerMemoryStore) int64 {
		return int64(int32(b.address.constant-a.address.constant)) + int64(b.offset) - int64(a.offset)
	}
	for i, a := range p.stores[:p.storeN] {
		if p.mapLoads && a.stride == 16 {
			continue
		}
		if a.stride == 4 || storePartner[i] >= 0 {
			continue
		}
		if a.stride != 8 {
			return
		}
		for j := i + 1; j < p.storeN; j++ {
			b := p.stores[j]
			delta := storeDelta(a, b)
			if storePartner[j] < 0 && b.stride == 8 && a.address.coeff == b.address.coeff && (delta == 4 || delta == -4) {
				storePartner[i], storePartner[j] = j, i
				break
			}
		}
		if storePartner[i] < 0 {
			return
		}
	}
	q.compactScratch = p.counted && !p.mapStores
	q.outputN = p.storeN
	for i := 0; i < p.storeN; i++ {
		q.outputRoots[i] = p.stores[i].value
	}
	v, ok := f.planPureVectors(&q)
	if !ok {
		return
	}
	// Coalesce only equal affine coefficients and strides. The modulo-i32
	// constant difference is merely a candidate: runtime equality below proves
	// the relationship after widening memory immediates.
	var parent [8]int
	var loadRangeFirst, loadRangeLast [8]int
	var displacement [8]int32
	var active [8]bool
	for i, load := range p.loads[:p.loadN] {
		parent[i] = i
		loadRangeFirst[i], loadRangeLast[i] = i, i
		for j := 0; j < i; j++ {
			base := p.loads[j]
			if parent[j] != j || base.stride != load.stride || base.address.coeff != load.address.coeff {
				continue
			}
			delta := int64(int32(load.address.constant-base.address.constant)) + int64(load.offset) - int64(base.offset)
			if delta < -1<<31 || delta > 1<<31-33 {
				continue
			}
			parent[i], displacement[i] = j, int32(delta)
			break
		}
		if v.reachable[load.node] {
			active[parent[i]] = true
		}
	}
	if p.mapLoads {
		var streams [8]integerMemoryStore
		for i, l := range p.loads[:p.loadN] {
			streams[i] = integerMemoryStore{address: l.address, offset: l.offset, stride: l.stride}
		}
		groups, valid := integerMemoryQuads(streams[:p.loadN])
		if !valid {
			return
		}
		for i, g := range groups[:p.loadN] {
			if g[0] < 0 {
				continue
			}
			// The final 128-bit row starts 112 bytes after the first. Keep
			// every signed x86 displacement representable before emission.
			if int64(displacement[g[0]])+112 > 1<<31-1 {
				return
			}
			var nodes [4]uint8
			for k, j := range g {
				if !v.reachable[p.loads[j].node] || parent[j] != parent[i] {
					return
				}
				nodes[k] = uint8(p.loads[j].node + 1)
			}
			q.loadQuads[p.loads[i].node] = nodes
			if f.opt(optIntegerMapGroupGuard) {
				loadRangeFirst[i], loadRangeLast[i] = g[0], g[3]
			}
		}
		v, ok = f.planPureVectors(&q)
		if !ok {
			return
		}
	}
	if wide && f.opt(optIntegerPairedLoads) {
		for i, a := range p.loads[:p.loadN] {
			if a.stride != 8 || !v.reachable[a.node] || q.loadPairs[a.node] != 0 {
				continue
			}
			for j := i + 1; j < p.loadN; j++ {
				b := p.loads[j]
				delta := int64(displacement[j]) - int64(displacement[i])
				if parent[i] != parent[j] || !v.reachable[b.node] || q.loadPairs[b.node] != 0 || (delta != 4 && delta != -4) {
					continue
				}
				q.loadPairs[a.node], q.loadPairs[b.node] = uint8(b.node+1), uint8(a.node+1)
				break
			}
		}
		if paired, fits := f.planPureVectors(&q); fits {
			v = paired
		} else {
			q.loadPairs = [128]uint8{}
		}
	}
	block := f.pinned.union(f.pinnedLocalMask).union(f.reserved)
	var gp [3]Reg
	n := 0
	for _, reg := range gpAlloc {
		if !block.has(reg) && f.regUser[reg] == nil {
			gp[n] = reg
			block = block.add(reg)
			n++
			if n == len(gp) {
				break
			}
		}
	}
	if n != len(gp) {
		return
	}
	count, tmp, other := gp[0], gp[1], gp[2]
	oldPinned, oldFPinned := f.pinned, f.fpinned
	for _, reg := range gp {
		f.pinned = f.pinned.add(reg)
	}
	f.fpinned = f.fpinned.union(v.mask)
	f.cpuHas(features)
	if p.counted {
		f.pureReduceFrameUsed = true
	}
	slot := f.maxSpill
	f.maxSpill += p.loadN + 2*p.storeN
	if p.mapLoads {
		f.maxSpill += p.loadN
	}
	off := func(i int) int32 { return f.spillOff(slot + i) }
	loadOff := func(i int) int32 { return off(2*p.storeN + i) }
	loadEnd := func(i int) int32 { return off(2*p.storeN + p.loadN + i) }
	// Recover homes before any conditional edge, so fallback sees the same state.
	for local := 0; local < p.locals; local++ {
		if f.localType[local] != mtI32 {
			continue
		}
		if _, _, pin := f.pinReg(local); pin {
			f.recoverLocal(local)
		} else if p.written&(1<<local) != 0 && f.locals[local].state == lsConstZero {
			f.a.XorSelf32(tmp)
			f.storeFrameInt(f.localAddr(local), tmp, mtI32)
			f.locals[local].state = lsMem
		}
	}
	read := func(local int, dst Reg) {
		if reg, _, pin := f.pinReg(local); pin {
			f.moveInt(dst, reg, mtI32)
		} else if f.locals[local].state == lsConstZero {
			f.a.XorSelf32(dst)
		} else {
			f.loadFrameInt(dst, f.localAddr(local), mtI32)
		}
	}
	write := func(local int, src Reg) {
		if reg, _, pin := f.pinReg(local); pin {
			f.moveInt(reg, src, mtI32)
		}
		f.storeFrameInt(f.localAddr(local), src, mtI32)
	}
	affine := func(a integerMemoryAddress, dst, scratch Reg) {
		f.a.MovImm32(dst, int32(a.constant))
		for local, c := range a.coeff {
			if c == 0 {
				continue
			}
			read(local, scratch)
			if c != 1 {
				f.a.ImulRI(scratch, int32(c), false)
			}
			f.a.Add32(dst, scratch)
		}
	}
	var fails [160]int
	failN := 0
	fail := func(cc Cond) { fails[failN] = f.a.JccPlaceholder(cc); failN++ }
	limit, _ := p.affine(p.limit)
	affine(limit, count, tmp)
	read(p.induction, tmp)
	f.a.Cmp32(count, tmp)
	fail(condBE)
	f.a.Sub32(count, tmp)
	if p.step != 1 {
		f.a.TestImm(count, p.step-1, false)
		fail(condNE)
		f.a.ShiftImm(5, count, byte(bits.TrailingZeros32(p.step)), false)
	}
	f.a.AluRI(7, count, lanes, false)
	fail(condB)
	// Establish every store range before the first write. Failing any proof
	// enters the original scalar body, preserving its partial writes on traps.
	for i, s := range p.stores[:p.storeN] {
		affine(s.address, tmp, other)
		f.a.Store64(RSP, off(i), tmp)
		f.a.MovRegReg32(other, count)
		f.a.AluRI(5, other, 1, true)
		f.a.ImulRI(other, int32(s.stride), true)
		f.a.AluRI(0, other, 4, true)
		f.a.Add64(other, tmp)
		f.a.MovImm64(tmp, 1<<32)
		f.a.Cmp64(other, tmp)
		fail(condA)
		if s.offset != 0 {
			f.a.MovImm64(tmp, s.offset)
			f.a.Add64(other, tmp)
		}
		f.a.AluRM(cmpRMcode, other, RBX, -bdCurBytes, true)
		fail(condA)
		f.a.Store64(RSP, off(p.storeN+i), other)
		if s.offset != 0 {
			f.a.Load64(other, RSP, off(i))
			f.a.Add64(other, tmp)
			f.a.Store64(RSP, off(i), other)
		}
	}
	for i, a := range p.stores[:p.storeN] {
		if g := storeQuads[i]; g[0] == i {
			for k := 1; k < 4; k++ {
				f.a.Load64(tmp, RSP, off(i))
				f.a.AluRI(0, tmp, int32(k*4), true)
				f.a.AluRM(cmpRMcode, tmp, RSP, off(g[k]), true)
				fail(condNE)
			}
		}
		if j := storePartner[i]; j > i {
			f.a.Load64(tmp, RSP, off(i))
			f.a.AluRI(0, tmp, int32(storeDelta(a, p.stores[j])), true)
			f.a.AluRM(cmpRMcode, tmp, RSP, off(j), true)
			fail(condNE)
		}
		for j := i + 1; j < p.storeN; j++ {
			if integerStoreDisjointResidues(a, p.stores[j]) {
				continue
			}
			f.a.Load64(tmp, RSP, off(p.storeN+i))
			f.a.AluRM(cmpRMcode, tmp, RSP, off(j), true)
			disjoint := f.a.JccPlaceholder(condBE)
			f.a.Load64(tmp, RSP, off(p.storeN+j))
			f.a.AluRM(cmpRMcode, tmp, RSP, off(i), true)
			fail(condA)
			f.a.PatchRel32(disjoint, f.a.Len())
		}
	}
	for i, load := range p.loads[:p.loadN] {
		affine(load.address, tmp, other)
		f.a.Store64(RSP, loadOff(i), tmp)
		f.a.MovRegReg32(other, count)
		f.a.AluRI(5, other, 1, true)
		f.a.ImulRI(other, int32(load.stride), true)
		// Stride-eight packing reads two full vector blocks. Guard its four
		// extra bytes instead of silently overreading the final scalar element.
		width := int32(4)
		if load.stride == 8 {
			width = 8
		}
		f.a.AluRI(0, other, width, true)
		f.a.Add64(other, tmp)
		f.a.MovImm64(tmp, 1<<32)
		f.a.Cmp64(other, tmp)
		fail(condA)
		if load.offset != 0 {
			f.a.MovImm64(tmp, load.offset)
			f.a.Add64(other, tmp)
		}
		f.a.AluRM(cmpRMcode, other, RBX, -bdCurBytes, true)
		fail(condA)
		if p.mapLoads {
			f.a.Store64(RSP, loadEnd(i), other)
		}
		if load.offset != 0 {
			f.a.Load64(other, RSP, loadOff(i))
			f.a.Add64(other, tmp)
			f.a.Store64(RSP, loadOff(i), other)
		}
	}
	// Reject every overlapping load/store range, including same-index aliases.
	// All checks precede guest reads and writes; the original scalar loop owns
	// ordering and partial-write semantics whenever any range proof fails.
	if p.mapLoads {
		for i := 0; i < p.loadN; i++ {
			if loadRangeFirst[i] != i {
				continue
			}
			for j := 0; j < p.storeN; j++ {
				storeLast := j
				if g := storeQuads[j]; f.opt(optIntegerMapGroupGuard) && g[0] >= 0 {
					if g[0] != j {
						continue
					}
					storeLast = g[3]
				}
				// Every member range lies inside these group endpoints once
				// the unchanged address equalities pass. Those equalities are
				// checked before any guest access, even if emitted later here.
				f.stats.peep("integer-map-overlap-guard")
				f.a.Load64(tmp, RSP, loadEnd(loadRangeLast[i]))
				f.a.AluRM(cmpRMcode, tmp, RSP, off(j), true)
				disjoint := f.a.JccPlaceholder(condBE)
				f.a.Load64(tmp, RSP, off(p.storeN+storeLast))
				f.a.AluRM(cmpRMcode, tmp, RSP, loadOff(i), true)
				fail(condA)
				f.a.PatchRel32(disjoint, f.a.Len())
			}
		}
	}
	for i := 0; i < p.loadN; i++ {
		if parent[i] == i {
			continue
		}
		f.a.Load64(tmp, RSP, loadOff(i))
		f.a.Load64(other, RSP, loadOff(parent[i]))
		if displacement[i] != 0 {
			f.a.AluRI(0, other, displacement[i], true)
		}
		f.a.Cmp64(tmp, other)
		fail(condNE)
		f.stats.peep("integer-memory-coalesced-stream")
	}
	for i := 0; i < p.n; i++ {
		if !v.reachable[i] {
			continue
		}
		node := p.nodes[i]
		if node.op != 0x20 && node.op != 0x41 {
			continue
		}
		if node.op == 0x41 {
			duplicate := false
			for j := 0; j < i; j++ {
				if v.reachable[j] && v.regs[j] == v.regs[i] {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			f.a.MovImm32(tmp, int32(node.v))
		} else {
			read(node.a, tmp)
		}
		f.a.MovGprToXmm(v.regs[i], tmp, false)
		f.a.Pshufd(v.regs[i], v.regs[i], 0)
		if i < p.locals && p.steps[i] != 0 {
			for lane := byte(1); lane < 4; lane++ {
				f.a.AluRI(0, tmp, int32(p.steps[i]), false)
				f.a.Pinsrd(v.regs[i], tmp, lane)
			}
		}
		if wide {
			high := v.regs[i]
			if i < p.locals && p.steps[i] != 0 {
				f.a.MovImm32(tmp, int32(p.steps[i]*4))
				f.a.MovGprToXmm(v.temp, tmp, false)
				f.a.Pshufd(v.temp, v.temp, 0)
				f.a.VPaddd(v.temp, v.regs[i], v.temp)
				high = v.temp
			}
			f.a.YInsertI128(v.regs[i], v.regs[i], high, 1)
		}
	}
	f.a.MovImm32(tmp, int32(p.step*uint32(lanes)))
	f.a.MovGprToXmm(v.stride, tmp, false)
	f.a.Pshufd(v.stride, v.stride, 0)
	if wide {
		f.a.YInsertI128(v.stride, v.stride, v.stride, 1)
	}
	for i := 0; i < p.reductionN; i++ {
		f.a.VPxor(v.acc[i], v.acc[i], v.acc[i])
	}
	var streams [128]int
	for i, load := range p.loads[:p.loadN] {
		streams[load.node] = i
	}
	// After initialization the two scratch GPRs are free for advancing stream
	// pointers. Invariant loads need a scratch GPR, and higher stream pressure
	// retains stack-based pointers rather than relinquishing unrelated pins.
	resident := true
	groups := 0
	for i, on := range active {
		if on {
			groups++
			resident = resident && p.loads[i].stride != 0
		}
	}
	resident = resident && groups <= 2 && !p.mapStores
	var streamReg [8]Reg
	for i := range streamReg {
		streamReg[i] = regNone
	}
	if resident {
		next := 1
		for i, on := range active {
			if !on {
				continue
			}
			streamReg[i] = gp[next]
			next++
			f.a.Load64(streamReg[i], RSP, loadOff(i))
			f.stats.peep("integer-memory-register-stream")
		}
	}
	loop := f.a.Len()
	add, sub, mul := f.a.VPaddd, f.a.VPsubd, f.a.VPmulld
	and, or, xor := f.a.VPand, f.a.VPor, f.a.VPxor
	shl, shr := f.a.VPslldImm, f.a.VPsrldImm
	if wide {
		add, sub, mul = f.a.YPaddd, f.a.YPsubd, f.a.YPmulld
		and, or, xor = f.a.YPand, f.a.YPor, f.a.YPxor
		shl, shr = f.a.YPslldImm, f.a.YPsrldImm
	}
	for i := 0; i < p.n; i++ {
		if !v.reachable[i] {
			continue
		}
		node := p.nodes[i]
		dst, a, b := v.regs[i], v.regs[node.a], v.regs[node.b]
		switch node.op {
		case 0x28:
			if group := q.loadQuads[i]; group[0] != 0 {
				leader := int(group[0]) - 1
				var regs [4]Reg
				for k, x := range group {
					regs[k] = v.regs[int(x)-1]
					if int(x)-1 < leader {
						leader = int(x) - 1
					}
				}
				if i != leader {
					continue
				}
				index := streams[int(group[0])-1]
				f.a.Load64(tmp, RSP, loadOff(parent[index]))
				disp := displacement[index]
				// Each input has rows k and k+4 in its two 128-bit lanes.
				// A lane-local transpose then produces natural iteration order.
				for row := 0; row < 4; row++ {
					d := regs[(row+2)%4]
					f.a.VMovdquLoadIdx(d, RBX, tmp, disp+int32(row*16))
					f.a.VMovdquLoadIdx(v.temp, RBX, tmp, disp+int32((row+4)*16))
					f.a.YInsertI128(d, d, v.temp, 1)
				}
				f.a.YShufps(v.temp, regs[2], regs[3], 0x44)
				f.a.YShufps(v.pair, regs[2], regs[3], 0xee)
				f.a.YShufps(regs[2], regs[0], regs[1], 0x44)
				f.a.YShufps(regs[3], regs[0], regs[1], 0xee)
				f.a.YShufps(regs[0], v.temp, regs[2], 0x88)
				f.a.YShufps(regs[1], v.temp, regs[2], 0xdd)
				f.a.YShufps(regs[2], v.pair, regs[3], 0x88)
				f.a.YShufps(regs[3], v.pair, regs[3], 0xdd)
				f.stats.peep("integer-map-quad-loads")
				continue
			}
			peer := int(q.loadPairs[i]) - 1
			if peer >= 0 && peer < i {
				continue
			}
			index := streams[i]
			load := p.loads[index]
			base := streamReg[parent[index]]
			disp := displacement[index]
			if base == regNone {
				base = tmp
				f.a.Load64(base, RSP, loadOff(parent[index]))
			}
			if peer > i {
				peerDisp := displacement[streams[peer]]
				even, odd := dst, v.regs[peer]
				if peerDisp < disp {
					disp = peerDisp
					even, odd = odd, even
				}
				// Both original ranges and the runtime equality establishing the
				// displacement were guarded above, before any guest load.
				f.a.YMovdquLoadIdx(even, RBX, base, disp)
				f.a.YMovdquLoadIdx(v.temp, RBX, base, disp+32)
				f.a.YShufps(odd, even, v.temp, 0xdd)
				f.a.YShufps(even, even, v.temp, 0x88)
				f.a.YPermq(odd, odd, 0xd8)
				f.a.YPermq(even, even, 0xd8)
				f.stats.peep("integer-paired-loads")
				continue
			}
			switch load.stride {
			case 0:
				f.a.LoadIdx(other, RBX, base, disp, 4, false, false)
				f.a.MovGprToXmm(dst, other, false)
				f.a.Pshufd(dst, dst, 0)
				if wide {
					f.a.YInsertI128(dst, dst, dst, 1)
				}
			case 4:
				if wide {
					f.a.YMovdquLoadIdx(dst, RBX, base, disp)
				} else {
					f.a.VMovdquLoadIdx(dst, RBX, base, disp)
				}
			case 8:
				if wide {
					f.a.YMovdquLoadIdx(dst, RBX, base, disp)
					f.a.YMovdquLoadIdx(v.temp, RBX, base, disp+32)
					f.a.YShufps(dst, dst, v.temp, 0x88)
					// SHUFPS packs each 128-bit lane independently. Restore
					// iteration order by exchanging the middle qwords.
					f.a.YPermq(dst, dst, 0xd8)
				} else {
					f.a.VMovdquLoadIdx(dst, RBX, base, disp)
					f.a.VMovdquLoadIdx(v.temp, RBX, base, disp+16)
					f.a.SseMapRRI(0, 0, 0xc6, dst, v.temp, 0x88)
				}
			}
		case 0x6a:
			add(dst, a, b)
		case 0x6b:
			sub(dst, a, b)
		case 0x6c:
			mul(dst, a, b)
		case 0x71:
			and(dst, a, b)
		case 0x72:
			or(dst, a, b)
		case 0x73:
			xor(dst, a, b)
		case 0x74:
			if p.nodes[node.b].op == 0x41 {
				shl(dst, a, byte(p.nodes[node.b].v&31))
			} else {
				shl(v.temp, b, 27)
				shr(v.temp, v.temp, 27)
				f.a.YPsllvd(dst, a, v.temp)
			}
		case 0x76:
			if p.nodes[node.b].op == 0x41 {
				shr(dst, a, byte(p.nodes[node.b].v&31))
			} else {
				shl(v.temp, b, 27)
				shr(v.temp, v.temp, 27)
				f.a.YPsrlvd(dst, a, v.temp)
			}
		case 0x46, 0x47:
			f.a.YPcmpeqd(dst, a, b)
			if node.op == 0x47 {
				xor(v.temp, v.temp, v.temp)
				f.a.YPcmpeqd(dst, dst, v.temp)
			}
			shr(dst, dst, 31)
		case 0x1b:
			xor(v.temp, v.temp, v.temp)
			f.a.YPcmpeqd(v.temp, v.regs[node.v], v.temp)
			f.a.YBlendvps(dst, a, b, v.temp)
		}
	}
	for i, s := range p.stores[:p.storeN] {
		if g := storeQuads[i]; g[0] >= 0 {
			if g[0] != i {
				continue
			}
			f.a.Load64(tmp, RSP, off(i))
			a, b, c, d := v.regs[p.stores[g[0]].value], v.regs[p.stores[g[1]].value], v.regs[p.stores[g[2]].value], v.regs[p.stores[g[3]].value]
			for row := 0; row < 4; row++ {
				unpack := f.a.YPunpckldq
				if row >= 2 {
					unpack = f.a.YPunpckhdq
				}
				unpack(v.temp, a, b)
				unpack(v.pair, c, d)
				imm := byte(0x44)
				if row&1 != 0 {
					imm = 0xee
				}
				f.a.YShufps(v.temp, v.temp, v.pair, imm)
				f.a.VMovdquStoreIdx(RBX, tmp, v.temp, int32(row*16))
				f.a.YExtractI128(v.temp, v.temp, 1)
				f.a.VMovdquStoreIdx(RBX, tmp, v.temp, int32((row+4)*16))
			}
			f.stats.peep("integer-map-quad-stores")
			continue
		}
		j := storePartner[i]
		if j >= 0 && j < i {
			continue
		}
		if s.stride == 4 {
			f.a.Load64(tmp, RSP, off(i))
			f.a.YMovdquStoreIdx(RBX, tmp, v.regs[s.value], 0)
		} else {
			low, high := i, j
			if storeDelta(s, p.stores[j]) < 0 {
				low, high = j, i
			}
			f.a.Load64(tmp, RSP, off(low))
			a, b := v.regs[p.stores[low].value], v.regs[p.stores[high].value]
			f.a.YPunpckldq(v.temp, a, b)
			f.a.VMovdquStoreIdx(RBX, tmp, v.temp, 0)
			f.a.YExtractI128(v.temp, v.temp, 1)
			f.a.VMovdquStoreIdx(RBX, tmp, v.temp, 32)
			f.a.YPunpckhdq(v.temp, a, b)
			f.a.VMovdquStoreIdx(RBX, tmp, v.temp, 16)
			f.a.YExtractI128(v.temp, v.temp, 1)
			f.a.VMovdquStoreIdx(RBX, tmp, v.temp, 48)
		}
	}
	for i, s := range p.stores[:p.storeN] {
		f.a.Load64(tmp, RSP, off(i))
		f.a.AluRI(0, tmp, int32(s.stride*uint32(lanes)), true)
		f.a.Store64(RSP, off(i), tmp)
	}
	for i, on := range active {
		if !on || p.loads[i].stride == 0 {
			continue
		}
		reg := streamReg[i]
		if reg == regNone {
			reg = tmp
			f.a.Load64(reg, RSP, loadOff(i))
		}
		f.a.AluRI(0, reg, int32(p.loads[i].stride*uint32(lanes)), true)
		if !resident {
			f.a.Store64(RSP, loadOff(i), reg)
		}
	}
	for i := 0; i < p.reductionN; i++ {
		add(v.acc[i], v.acc[i], v.regs[p.reductions[i].term])
	}
	add(v.regs[p.induction], v.regs[p.induction], v.stride)
	for local := 0; local < p.locals; local++ {
		if local == p.induction || p.steps[local] == 0 || !v.reachable[local] {
			continue
		}
		stride := v.stride
		if p.steps[local] != p.step {
			f.a.MovImm32(tmp, int32(p.steps[local]*uint32(lanes)))
			f.a.MovGprToXmm(v.temp, tmp, false)
			f.a.Pshufd(v.temp, v.temp, 0)
			f.a.YInsertI128(v.temp, v.temp, v.temp, 1)
			stride = v.temp
		}
		add(v.regs[local], v.regs[local], stride)
	}
	f.a.AluRI(5, count, lanes, false)
	f.a.AluRI(7, count, lanes, false)
	f.a.PatchRel32(f.a.JccPlaceholder(condAE), loop)
	for i := 0; i < p.reductionN; i++ {
		if wide {
			f.a.YExtractI128(v.temp, v.acc[i], 1)
			f.a.VPaddd(v.acc[i], v.acc[i], v.temp)
		}
		f.a.Pshufd(v.temp, v.acc[i], 0x4e)
		f.a.VPaddd(v.pair, v.acc[i], v.temp)
		f.a.Pshufd(v.temp, v.pair, 0xb1)
		f.a.VPaddd(v.pair, v.pair, v.temp)
		f.a.MovXmmToGpr(tmp, v.pair, false)
		read(p.reductions[i].local, other)
		f.a.Add32(tmp, other)
		write(p.reductions[i].local, tmp)
	}
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 {
			continue
		}
		reduction := false
		for i := 0; i < p.reductionN; i++ {
			if p.reductions[i].local == local {
				reduction = true
			}
		}
		if reduction {
			continue
		}
		if p.steps[local] != 0 {
			f.a.MovXmmToGpr(tmp, v.regs[local], false)
		} else if p.values[local] < p.locals && p.steps[p.values[local]] != 0 {
			f.a.MovXmmToGpr(tmp, v.regs[p.values[local]], false)
			f.a.AluRI(5, tmp, int32(p.steps[p.values[local]]), false)
		} else {
			src := v.regs[p.values[local]]
			if wide {
				f.a.YExtractI128(v.temp, src, 1)
				src = v.temp
			}
			f.a.Pextrd(tmp, src, 3)
		}
		write(local, tmp)
	}
	if wide {
		f.a.VZeroUpper()
		f.stats.peep("integer-memory-vector-avx2")
	}
	f.a.TestSelf(count, false)
	finished := f.a.JccPlaceholder(condE)
	tail := -1
	if integerMemoryAssertFast {
		tail = f.a.JmpPlaceholder()
	}
	for _, site := range fails[:failN] {
		f.a.PatchRel32(site, f.a.Len())
	}
	if integerMemoryAssertFast {
		f.trapAlways(trapUnreachable)
		f.a.PatchRel32(tail, f.a.Len())
	}
	f.pinned, f.fpinned = oldPinned, oldFPinned
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 {
			continue
		}
		if _, _, pin := f.pinReg(local); pin {
			f.locals[local].state = lsReg
		} else {
			f.locals[local].state = lsMem
		}
	}
	f.integerMemory = &pureReduceJoin{depth: len(f.ctrl) + 1, site: finished, written: p.written}
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.stats.peep("integer-memory-vector")
	if p.mapStores {
		f.stats.peep("integer-store-vector")
	}
	if p.mapLoads {
		f.stats.peep("integer-map-vector")
	}
	if p.counted && !p.mapStores {
		f.stats.peep("integer-counted-vector")
	}
}

func (f *fn) finishIntegerMemoryLoop(depth int) {
	if f.integerMemory == nil || f.integerMemory.depth != depth {
		return
	}
	f.a.PatchRel32(f.integerMemory.site, f.a.Len())
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.integerMemory = nil
}
