//go:build amd64

package amd64

import (
	"math/bits"
	"os"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
)

var pureReduceEnabled = os.Getenv("WAGO_AMD64_PURE_REDUCE") == "1"

// This is opt-in until corpus measurements qualify its execution/compile tradeoff.
func (f *fn) emitPureReduce(r *wasm.Reader) {
	if !f.opt(optPureLoopVector) || f.pureReduce != nil || f.usesCalls || f.interruptible || f.moduleEH || f.localBase != 0 || len(f.customInstructions) != 0 || len(f.intervalReg) != 0 || f.depth() != 0 || f.pinRelinquished {
		return
	}
	features := shared.AMD64AVX | shared.AMD64SSE41
	if !f.sc.amd64Features.Has(features) {
		return
	}
	wide := f.sc.amd64Features.Has(shared.AMD64AVX2)
	lanes := int32(4)
	if wide {
		lanes = 8
		features |= shared.AMD64AVX2
	}
	p, ok := f.inspectPureReduce(r)
	if !ok {
		return
	}
	// Accumulate in dword lanes only for a bounded number of vector rounds.
	// Each lane sees at most maxTerm per round, so this division proves that
	// widening at the chunk boundary precedes unsigned overflow. Require enough
	// rounds to amortize the extra counter and horizontal merge.
	rounds := ^uint32(0) / uint32(lanes)
	if p.maxTerm != 0 {
		rounds = min(rounds, ^uint32(0)/p.maxTerm)
	}
	narrow := rounds >= 16
	v, ok := f.planPureVectors(&p)
	if !ok {
		return
	}
	// No allocation or spilling during the preflight.
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
	// The vector bypass explicitly refreshes local slots even if every local
	// has a pin. The usual register-homed leaf proof cannot elide this frame.
	f.pureReduceFrameUsed = true
	read := func(local int, dst Reg) {
		if reg, _, pin := f.pinReg(local); pin {
			f.recoverLocal(local)
			f.moveInt(dst, reg, f.localType[local])
		} else {
			f.loadFrameInt(dst, f.localAddr(local), f.localType[local])
		}
	}
	write := func(local int, src Reg) {
		if reg, _, pin := f.pinReg(local); pin {
			f.moveInt(reg, src, f.localType[local])
		}
		// Both homes remain valid on the bypass edge, including loop-end merges.
		f.storeFrameInt(f.localAddr(local), src, f.localType[local])
	}
	for local := 0; local < p.locals; local++ {
		if (p.written&(1<<local) != 0 || v.reachable[local]) && f.locals[local].state == lsConstZero {
			f.materializeZeroLocal(local, true)
		}
	}
	read(p.counter, count)
	f.a.AluRI(7, count, lanes, false)
	toScalar := f.a.JccPlaceholder(condB)
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
		if i == p.induction {
			for lane := byte(1); lane < 4; lane++ {
				f.a.AluRI(0, tmp, int32(p.step), false)
				f.a.Pinsrd(v.regs[i], tmp, lane)
			}
		}
		if wide {
			high := v.regs[i]
			if i == p.induction {
				f.a.MovImm32(tmp, int32(p.step*4))
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
	chunk := f.a.Len()
	if narrow {
		f.a.MovImm32(other, int32(rounds))
	}
	for i := 0; i < p.nReductions; i++ {
		f.a.VPxor(v.acc[i], v.acc[i], v.acc[i])
	}
	loop := f.a.Len()
	add, sub, mul := f.a.VPaddd, f.a.VPsubd, f.a.VPmulld
	and, or, xor := f.a.VPand, f.a.VPor, f.a.VPxor
	shl, shr, mov := f.a.VPslldImm, f.a.VPsrldImm, f.a.VMovdqu
	if wide {
		add, sub, mul = f.a.YPaddd, f.a.YPsubd, f.a.YPmulld
		and, or, xor = f.a.YPand, f.a.YPor, f.a.YPxor
		shl, shr, mov = f.a.YPslldImm, f.a.YPsrldImm, f.a.YMovdqu
	}
	for i := 0; i < p.n; i++ {
		if !v.reachable[i] {
			continue
		}
		node := p.nodes[i]
		dst, a, b := v.regs[i], v.regs[node.a], v.regs[node.b]
		switch node.op {
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
			shl(dst, a, byte(p.nodes[node.b].v&31))
		case 0x76:
			shr(dst, a, byte(p.nodes[node.b].v&31))
		case 0xad:
			mov(dst, a)
		}
	}
	for i := 0; i < p.nReductions; i++ {
		term := v.regs[p.reductions[i].term]
		if narrow {
			add(v.acc[i], v.acc[i], term)
			continue
		}
		if wide {
			f.a.YPmovzxdq(v.pair, term)
			f.a.YExtractI128(v.temp, term, 1)
			f.a.YPmovzxdq(v.temp, v.temp)
			f.a.YPaddq(v.pair, v.pair, v.temp)
			f.a.YPaddq(v.acc[i], v.acc[i], v.pair)
			continue
		}
		f.a.SseMapRR(0x66, 0x38, 0x35, v.pair, term) // pmovzxdq low two lanes
		f.a.Pshufd(v.temp, term, 0x4e)
		f.a.SseMapRR(0x66, 0x38, 0x35, v.temp, v.temp)
		f.a.VPaddq(v.pair, v.pair, v.temp)
		f.a.VPaddq(v.acc[i], v.acc[i], v.pair)
	}
	add(v.regs[p.induction], v.regs[p.induction], v.stride)
	f.a.AluRI(5, count, lanes, false)
	chunkFull := -1
	if narrow {
		f.a.AluRI(5, other, 1, false)
		chunkFull = f.a.JccPlaceholder(condE)
	}
	f.a.AluRI(7, count, lanes, false)
	f.a.PatchRel32(f.a.JccPlaceholder(condAE), loop)
	if chunkFull != -1 {
		f.a.PatchRel32(chunkFull, f.a.Len())
	}
	for i := 0; i < p.nReductions; i++ {
		sum := v.acc[i]
		if narrow {
			if wide {
				f.a.YPmovzxdq(v.pair, sum)
				f.a.YExtractI128(v.temp, sum, 1)
				f.a.YPmovzxdq(v.temp, v.temp)
				f.a.YPaddq(v.pair, v.pair, v.temp)
			} else {
				f.a.SseMapRR(0x66, 0x38, 0x35, v.pair, sum)
				f.a.Pshufd(v.temp, sum, 0x4e)
				f.a.SseMapRR(0x66, 0x38, 0x35, v.temp, v.temp)
				f.a.VPaddq(v.pair, v.pair, v.temp)
			}
			sum = v.pair
		}
		if wide {
			f.a.YExtractI128(v.temp, sum, 1)
			f.a.VPaddq(v.pair, sum, v.temp)
			sum = v.pair
		}
		f.a.Pshufd(v.temp, sum, 0x4e)
		f.a.VPaddq(v.pair, sum, v.temp)
		f.a.MovXmmToGpr(tmp, v.pair, true)
		read(p.reductions[i].local, other)
		f.a.AluRR(0x01, tmp, other, true)
		write(p.reductions[i].local, tmp)
	}
	if narrow {
		f.a.AluRI(7, count, lanes, false)
		f.a.PatchRel32(f.a.JccPlaceholder(condAE), chunk)
		f.stats.peep("pure-loop-bounded-accumulate")
	}
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 {
			continue
		}
		isAcc := false
		for i := 0; i < p.nReductions; i++ {
			if p.reductions[i].local == local {
				isAcc = true
			}
		}
		if isAcc {
			continue
		}
		if local == p.counter {
			write(local, count)
			continue
		}
		if local == p.induction {
			f.a.MovXmmToGpr(tmp, v.regs[p.induction], false)
		} else if p.values[local] == p.induction {
			f.a.MovXmmToGpr(tmp, v.regs[p.induction], false)
			f.a.AluRI(5, tmp, int32(p.step), false)
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
		f.stats.peep("pure-loop-vector-avx2")
	}
	f.a.TestSelf(count, false)
	finished := f.a.JccPlaceholder(condE)
	f.a.PatchRel32(toScalar, f.a.Len())
	f.pinned, f.fpinned = oldPinned, oldFPinned
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) != 0 {
			// The short-count edge retained its entry homes; don't strengthen
			// that state merely because the vector edge refreshed both homes.
			if _, _, pin := f.pinReg(local); pin {
				f.locals[local].state = lsReg
			} else {
				f.locals[local].state = lsMem
			}
		}
	}
	f.pureReduce = &pureReduceJoin{depth: len(f.ctrl) + 1, site: finished, written: p.written}
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.stats.peep("pure-loop-vector")
}

func (f *fn) finishPureReduce(depth int) {
	if f.pureReduce == nil || f.pureReduce.depth != depth {
		return
	}
	f.a.PatchRel32(f.pureReduce.site, f.a.Len())
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.pureReduce = nil
}

// The bounded scan expands local tees into a DAG. Eligible bodies have one
// modular i32 induction, a countdown exit, and independent additive i64
// reductions. Widened products require a proven 32-bit result.
type pureReduceNode struct {
	op   byte
	a, b int
	v    uint32
}

type pureReduction struct{ local, term int }
type pureReducePlan struct {
	// Additional outputs, such as independent stored values, remain live until
	// the emitter consumes the complete vector group.
	outputRoots [8]int
	outputN     int
	// Nonzero entries name a paired load node plus one. Both outputs are born
	// at the earlier load, so allocation must extend the later output's life.
	loadPairs [128]uint8
	// Four-way interleaved loads share the birth of their earliest node.
	loadQuads [128][4]uint8
	// Counted i32 reductions can reuse the loop stride register for
	// their post-loop horizontal sum. Memory packing and i64 widening cannot.
	compactScratch                bool
	nodes                         [128]pureReduceNode
	values                        [32]int
	written                       uint32
	n, locals, counter, induction int
	step                          uint32
	maxTerm                       uint32
	reductions                    [4]pureReduction
	nReductions                   int
}
type pureReduceJoin struct {
	depth, site int
	written     uint32
}

func (f *fn) inspectPureReduce(r *wasm.Reader) (p pureReducePlan, ok bool) {
	if f.nLocals > 32 || len(f.localType) != f.nLocals {
		return
	}
	p.locals, p.n = f.nLocals, f.nLocals
	hasReductionType := false
	for i, typ := range f.localType {
		if typ != mtI32 && typ != mtI64 {
			return p, false
		}
		p.nodes[i] = pureReduceNode{op: 0x20, a: i}
		p.values[i] = i
		hasReductionType = hasReductionType || typ == mtI64
	}
	if !hasReductionType {
		return p, false
	}
	scan := *r
	start := scan.Offset()
	var stack [24]int
	depth := 0
	push := func(x int) bool {
		if depth == len(stack) {
			return false
		}
		stack[depth] = x
		depth++
		return true
	}
	node := func(v pureReduceNode) int {
		if p.n == len(p.nodes) {
			return -1
		}
		p.nodes[p.n] = v
		p.n++
		return p.n - 1
	}
	for scan.Offset()-start < 512 {
		op, err := scan.Byte()
		if err != nil {
			return p, false
		}
		switch op {
		case 0x20, 0x21, 0x22:
			x, err := scan.U32()
			if err != nil || int(x) >= p.locals {
				return p, false
			}
			if op == 0x20 {
				if !push(p.values[x]) {
					return p, false
				}
				continue
			}
			if depth == 0 {
				return p, false
			}
			p.values[x] = stack[depth-1]
			p.written |= 1 << x
			if op == 0x21 {
				depth--
			}
		case 0x41:
			v, err := scan.I32()
			if err != nil {
				return p, false
			}
			x := node(pureReduceNode{op: op, v: uint32(v)})
			if x < 0 || !push(x) {
				return p, false
			}
		case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x76, 0x7c, 0x7e:
			if depth < 2 {
				return p, false
			}
			depth -= 2
			x := node(pureReduceNode{op: op, a: stack[depth], b: stack[depth+1]})
			if x < 0 || !push(x) {
				return p, false
			}
		case 0xad:
			if depth == 0 {
				return p, false
			}
			x := node(pureReduceNode{op: op, a: stack[depth-1]})
			if x < 0 {
				return p, false
			}
			stack[depth-1] = x
		case 0x0d:
			label, err := scan.U32()
			if err != nil || label != 0 || depth != 1 {
				return p, false
			}
			end, err := scan.Byte()
			if err != nil || end != 0x0b {
				return p, false
			}
			p.counter = -1
			for local := 0; local < p.locals; local++ {
				if p.written&(1<<local) == 0 || f.localType[local] != mtI32 {
					continue
				}
				v := p.nodes[p.values[local]]
				if v.op != 0x6a && v.op != 0x6b {
					continue
				}
				a, b := v.a, v.b
				if v.op == 0x6a && p.nodes[a].op == 0x41 {
					a, b = b, a
				}
				if p.nodes[a].op != 0x20 || p.nodes[a].a != local || p.nodes[b].op != 0x41 {
					continue
				}
				step := p.nodes[b].v
				if v.op == 0x6b {
					step = -step
				}
				if step == 0xffffffff && p.values[local] == stack[0] {
					p.counter = local
				}
			}
			if p.counter < 0 {
				return p, false
			}
			p.induction = -1
			for local := 0; local < p.locals; local++ {
				if local == p.counter || p.written&(1<<local) == 0 || f.localType[local] != mtI32 {
					continue
				}
				v := p.nodes[p.values[local]]
				if v.op != 0x6a && v.op != 0x6b {
					continue
				}
				a, b := v.a, v.b
				if v.op == 0x6a && p.nodes[a].op == 0x41 {
					a, b = b, a
				}
				if p.nodes[a].op != 0x20 || p.nodes[a].a != local || p.nodes[b].op != 0x41 {
					continue
				}
				if p.induction >= 0 {
					return p, false
				}
				p.induction = local
				p.step = p.nodes[b].v
				if v.op == 0x6b {
					p.step = -p.step
				}
			}
			if p.induction < 0 {
				return p, false
			}
			return f.provePureReduce(p)
		default:
			return p, false
		}
	}
	return p, false
}

func (f *fn) provePureReduce(p pureReducePlan) (pureReducePlan, bool) {
	var valid [128]bool
	var bound [128]uint32
	for i := 0; i < p.n; i++ {
		v := p.nodes[i]
		switch v.op {
		case 0x20:
			valid[i] = f.localType[v.a] == mtI32 && v.a != p.counter && (v.a == p.induction || p.written&(1<<v.a) == 0)
			bound[i] = ^uint32(0)
		case 0x41:
			valid[i] = true
			bound[i] = v.v
		case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73:
			if !valid[v.a] || !valid[v.b] {
				continue
			}
			valid[i] = true
			bound[i] = ^uint32(0)
			if v.op == 0x71 {
				bound[i] = min(bound[v.a], bound[v.b])
			}
			if v.op == 0x72 || v.op == 0x73 {
				width := bits.Len32(bound[v.a] | bound[v.b])
				if width < 32 {
					bound[i] = (1 << width) - 1
				}
			}
			if v.op == 0x6a && uint64(bound[v.a])+uint64(bound[v.b]) <= 0xffffffff {
				bound[i] = bound[v.a] + bound[v.b]
			}
			if v.op == 0x6c && uint64(bound[v.a])*uint64(bound[v.b]) <= 0xffffffff {
				bound[i] = bound[v.a] * bound[v.b]
			}
		case 0x74, 0x76:
			if !valid[v.a] || p.nodes[v.b].op != 0x41 {
				continue
			}
			valid[i] = true
			bound[i] = ^uint32(0)
			if v.op == 0x76 {
				bound[i] = bound[v.a] >> (p.nodes[v.b].v & 31)
			}
		case 0xad:
			if valid[v.a] {
				valid[i] = true
				bound[i] = bound[v.a]
			}
		case 0x7e:
			if p.nodes[v.a].op != 0xad || p.nodes[v.b].op != 0xad || !valid[v.a] || !valid[v.b] || uint64(bound[v.a])*uint64(bound[v.b]) > 0xffffffff {
				continue
			}
			valid[i] = true
			bound[i] = bound[v.a] * bound[v.b]
		}
	}
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 || local == p.counter || local == p.induction {
			continue
		}
		x := p.values[local]
		v := p.nodes[x]
		if f.localType[local] == mtI64 && v.op == 0x7c {
			a, b := v.a, v.b
			if p.nodes[b].op == 0x20 && p.nodes[b].a == local {
				a, b = b, a
			}
			if p.nodes[a].op != 0x20 || p.nodes[a].a != local || !valid[b] || p.nReductions == len(p.reductions) {
				return p, false
			}
			p.reductions[p.nReductions] = pureReduction{local, b}
			p.maxTerm = max(p.maxTerm, bound[b])
			p.nReductions++
		} else if !valid[x] {
			return p, false
		}
	}
	if p.nReductions == 0 {
		return p, false
	}
	// Canonicalize widening nodes after proving full-precision products. The
	// emitted 32-bit terms are widened by pairwise addition into 64-bit lanes.
	for i := 0; i < p.n; i++ {
		v := &p.nodes[i]
		if v.op == 0x7e && valid[i] {
			v.op = 0x6c
			v.a = p.nodes[v.a].a
			v.b = p.nodes[v.b].a
		}
	}
	for i := 0; i < p.nReductions; i++ {
		x := p.reductions[i].term
		if p.nodes[x].op == 0xad {
			p.reductions[i].term = p.nodes[x].a
		}
	}
	for local := 0; local < p.locals; local++ {
		x := p.values[local]
		if p.nodes[x].op == 0xad {
			p.values[local] = p.nodes[x].a
		}
	}
	return p, true
}

type pureVectorLayout struct {
	regs               [128]Reg
	refs               [128]int
	reachable          [128]bool
	acc                [4]Reg
	pair, temp, stride Reg
	mask               regMask
}

// Preflight physical vector assignments before emission. Invariants remain
// resident; DAG temporaries release at their last use; final source-local roots
// stay live for lane extraction. Excess pressure declines without state changes.
func (f *fn) planPureVectors(p *pureReducePlan) (v pureVectorLayout, ok bool) {
	for i := range v.regs {
		v.regs[i] = regNone
	}
	var visit func(int)
	visit = func(x int) {
		if v.reachable[x] {
			return
		}
		v.reachable[x] = true
		n := p.nodes[x]
		if n.op != 0x20 && n.op != 0x41 && n.op != 0x28 {
			v.refs[n.a]++
			visit(n.a)
			if n.op != 0xad && ((n.op != 0x74 && n.op != 0x76) || p.nodes[n.b].op != 0x41) {
				v.refs[n.b]++
				visit(n.b)
			}
			if n.op == 0x1b {
				v.refs[n.v]++
				visit(int(n.v))
			}
		}
	}
	root := func(x int) { v.refs[x]++; visit(x) }
	root(p.induction)
	for _, x := range p.outputRoots[:p.outputN] {
		root(x)
	}
	for i := 0; i < p.nReductions; i++ {
		root(p.reductions[i].term)
	}
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 || local == p.counter || local == p.induction {
			continue
		}
		isAcc := false
		for i := 0; i < p.nReductions; i++ {
			if p.reductions[i].local == local {
				isAcc = true
			}
		}
		if !isAcc {
			root(p.values[local])
		}
	}
	inuse := f.fpinned.union(f.fpinnedLocalMask).union(f.fconstMask()).union(f.v128ConstMask())
	alloc := func() Reg {
		for r := Reg(0); r < 16; r++ {
			if !inuse.has(r) && f.fregUser[r] == nil {
				inuse = inuse.add(r)
				v.mask = v.mask.add(r)
				return r
			}
		}
		return regNone
	}
	v.pair, v.temp = alloc(), alloc()
	if p.compactScratch {
		v.stride = v.pair
	} else {
		v.stride = alloc()
	}
	if v.pair == regNone || v.temp == regNone || v.stride == regNone {
		return v, false
	}
	for i := 0; i < p.nReductions; i++ {
		v.acc[i] = alloc()
		if v.acc[i] == regNone {
			return v, false
		}
	}
	for i := 0; i < p.n; i++ {
		if !v.reachable[i] {
			continue
		}
		n := p.nodes[i]
		if n.op != 0x20 && n.op != 0x41 {
			continue
		}
		if n.op == 0x41 {
			for j := 0; j < i; j++ {
				if v.reachable[j] && p.nodes[j].op == 0x41 && p.nodes[j].v == n.v {
					v.regs[i] = v.regs[j]
					break
				}
			}
		}
		if v.regs[i] == regNone {
			v.regs[i] = alloc()
			if v.regs[i] == regNone {
				return v, false
			}
		}
	}
	refs := v.refs
	consume := func(x int) {
		refs[x]--
		if refs[x] == 0 && p.nodes[x].op != 0x20 && p.nodes[x].op != 0x41 {
			inuse = inuse.remove(v.regs[x])
		}
	}
	for i := 0; i < p.n; i++ {
		if !v.reachable[i] {
			continue
		}
		n := p.nodes[i]
		if n.op == 0x20 || n.op == 0x41 {
			continue
		}
		if peer := int(p.loadPairs[i]) - 1; peer >= 0 && peer < i {
			continue // assigned together with the earlier load
		}
		quad := p.loadQuads[i]
		if quad[0] != 0 {
			leader := int(quad[0]) - 1
			for _, x := range quad {
				if int(x)-1 < leader {
					leader = int(x) - 1
				}
			}
			if i != leader {
				continue
			}
		}
		// VEX reads both sources before writing its destination. Consume first
		// so a dying temporary can be reused even for noncommutative operations.
		if n.op != 0x28 {
			consume(n.a)
			if n.op != 0xad && ((n.op != 0x74 && n.op != 0x76) || p.nodes[n.b].op != 0x41) {
				consume(n.b)
			}
			if n.op == 0x1b {
				consume(int(n.v))
			}
		}
		v.regs[i] = alloc()
		if v.regs[i] == regNone {
			return v, false
		}
		if peer := int(p.loadPairs[i]) - 1; peer > i {
			v.regs[peer] = alloc()
			if v.regs[peer] == regNone {
				return v, false
			}
		}
		if quad[0] != 0 {
			for _, x := range quad {
				j := int(x) - 1
				if j == i {
					continue
				}
				v.regs[j] = alloc()
				if v.regs[j] == regNone {
					return v, false
				}
			}
		}
	}
	return v, true
}
