//go:build amd64

package amd64

import "github.com/wago-org/wago/src/core/compiler/wasm"

// Affine arithmetic here is modulo 2^32. A memory immediate remains separate:
// Wasm adds it without wrapping. This planner does not authorize unchecked
// loads; an emitter must guard the entire range and retain the scalar fallback.
type integerMemoryAddress struct {
	constant uint32
	coeff    [32]uint32
}

type integerMemoryLoad struct {
	node    int
	address integerMemoryAddress
	offset  uint64
	stride  uint32
}

// A store plan still requires full-range and pairwise overlap guards before
// any vector writes. It is not permission to reorder potentially aliasing stores.
type integerMemoryStore struct {
	address       integerMemoryAddress
	offset        uint64
	stride        uint32
	source, value int
}

// Group four adjacent i32 fields with equal affine coefficients. Widened
// runtime address equalities and full ranges remain the emitter's obligation.
func integerMemoryQuads(streams []integerMemoryStore) ([8][4]int, bool) {
	var groups [8][4]int
	for i := range groups {
		groups[i] = [4]int{-1, -1, -1, -1}
	}
	for i, a := range streams {
		if a.stride != 16 || groups[i][0] >= 0 {
			continue
		}
		g := [4]int{i, -1, -1, -1}
		for k := 1; k < 4; k++ {
			for j, b := range streams {
				if groups[j][0] < 0 && b.stride == 16 && a.address.coeff == b.address.coeff && int64(int32(b.address.constant-a.address.constant))+int64(b.offset)-int64(a.offset) == int64(k*4) {
					g[k] = j
					break
				}
			}
		}
		if g[1] < 0 || g[2] < 0 || g[3] < 0 {
			continue
		}
		for _, j := range g {
			groups[j] = g
		}
	}
	for i, s := range streams {
		if s.stride == 16 && groups[i][0] < 0 {
			return groups, false
		}
	}
	return groups, true
}

// With equal affine coefficients, differences modulo a power-of-two stride
// are fixed even when i32 address arithmetic wraps. Distinct four-byte residue
// intervals cannot alias. Full nonwrapping range guards are still mandatory.
func integerStoreDisjointResidues(a, b integerMemoryStore) bool {
	stride := a.stride
	if stride < 4 || stride != b.stride || stride&(stride-1) != 0 || a.address.coeff != b.address.coeff {
		return false
	}
	delta := (uint64(b.address.constant) + b.offset - uint64(a.address.constant) - a.offset) & uint64(stride-1)
	return delta >= 4 && delta <= uint64(stride)-4
}

type integerMemoryPlan struct {
	// Extended counted plans require the separate counted-vector feature gate.
	counted                      bool
	mapStores                    bool
	mapLoads                     bool
	stores                       [8]integerMemoryStore
	storeN                       int
	steps                        [32]uint32
	nodes                        [128]pureReduceNode
	values                       [32]int
	loads                        [8]integerMemoryLoad
	reductions                   [4]pureReduction
	n, locals, loadN, reductionN int
	induction, limit, endPC      int
	step, written                uint32
}

func (p *integerMemoryPlan) add(n pureReduceNode) int {
	if p.n == len(p.nodes) {
		return -1
	}
	id := p.n
	p.nodes[id] = n
	p.n++
	return id
}

func (f *fn) inspectIntegerMemoryLoop(r *wasm.Reader) (p integerMemoryPlan, ok bool) {
	return f.inspectIntegerLoop(r, false)
}

// inspectIntegerLoop also supports an extension for memory-free
// reductions, independent modular induction variables, predicates and selects.
// The ordinary memory path never admits these extended plans.
func (f *fn) inspectIntegerLoop(r *wasm.Reader, counted bool) (p integerMemoryPlan, ok bool) {
	return f.inspectIntegerLoopWithStores(r, counted, false)
}

// Store maps require a separately gated emitter with range and alias guards.
// Ordinary reduction paths call this with mapStores=false.
func (f *fn) inspectIntegerLoopWithStores(r *wasm.Reader, counted, mapStores bool) (p integerMemoryPlan, ok bool) {
	return f.inspectIntegerMapLoop(r, counted, mapStores, false)
}

// Load/store maps require the separately gated emitter to prove full ranges
// and load/store independence. Ordinary reduction/store paths never opt in.
func (f *fn) inspectIntegerMapLoop(r *wasm.Reader, counted, mapStores, mapLoads bool) (p integerMemoryPlan, ok bool) {
	p.counted = counted
	p.mapStores = mapStores
	p.mapLoads = mapLoads
	if mapLoads && (!counted || !mapStores) {
		return p, false
	}
	if f.nLocals > 32 || len(f.localType) != f.nLocals {
		return
	}
	p.n, p.locals = f.nLocals, f.nLocals
	for i := 0; i < p.locals; i++ {
		p.nodes[i] = pureReduceNode{op: 0x20, a: i}
		p.values[i] = i
	}
	scan := regionLoopReader(*r)
	var stack [24]int
	depth := 0
	for scan.HasNext() {
		op, err := scan.Byte()
		if err != nil {
			return p, false
		}
		look := scan
		var imm wasm.InstructionImmediate
		if f.classifier.ClassifyInto(&scan, op, &imm) != nil {
			return p, false
		}
		id := -1
		switch op {
		case 0x20, 0x21, 0x22:
			x := int(imm.Index)
			if x >= p.locals || f.localType[x] != mtI32 {
				return p, false
			}
			if op == 0x20 {
				id = p.values[x]
			} else {
				if depth == 0 {
					return p, false
				}
				p.values[x] = stack[depth-1]
				p.written |= 1 << x
				if op == 0x21 {
					depth--
				}
				continue
			}
		case 0x41:
			value, err := look.I32()
			if err != nil {
				return p, false
			}
			id = p.add(pureReduceNode{op: op, v: uint32(value)})
		case 0x28:
			if mapStores && !mapLoads || depth == 0 || imm.MemIndex != 0 || imm.MemOffset > 0xffffffff || p.loadN == len(p.loads) {
				return p, false
			}
			depth--
			id = p.add(pureReduceNode{op: op, a: stack[depth]})
			if id < 0 {
				return p, false
			}
			p.loads[p.loadN] = integerMemoryLoad{node: id, offset: imm.MemOffset}
			p.loadN++
		case 0x36:
			if !mapStores || depth < 2 || imm.MemIndex != 0 || imm.MemOffset > 0xffffffff || p.storeN == len(p.stores) {
				return p, false
			}
			depth -= 2
			p.stores[p.storeN] = integerMemoryStore{source: stack[depth], value: stack[depth+1], offset: imm.MemOffset}
			p.storeN++
			continue
		case 0x1b:
			if !counted || depth < 3 {
				return p, false
			}
			depth -= 3
			// Encode the third child in v, which is unused by non-constants.
			id = p.add(pureReduceNode{op: op, a: stack[depth], b: stack[depth+1], v: uint32(stack[depth+2])})
		case 0x46, 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x76, 0x47:
			if op == 0x46 && !counted {
				return p, false
			}
			if depth < 2 {
				return p, false
			}
			depth -= 2
			id = p.add(pureReduceNode{op: op, a: stack[depth], b: stack[depth+1]})
		case 0x0d:
			if imm.Index != 0 || depth != 1 || (!counted && p.loadN == 0) || p.nodes[stack[0]].op != 0x47 {
				return p, false
			}
			end, err := scan.Byte()
			if err != nil || end != 0x0b {
				return p, false
			}
			p.endPC = r.Offset() + scan.Offset()
			return f.proveIntegerMemoryLoop(p, stack[0])
		default:
			// No stores, calls, nested control, division or other possible effects.
			return p, false
		}
		if id < 0 || depth == len(stack) {
			return p, false
		}
		stack[depth] = id
		depth++
	}
	return p, false
}

// Build an affine form in topological order. Only compile-time multiplication
// and shifts preserve affinity; load-dependent addresses are never admitted.
func (p *integerMemoryPlan) affine(root int) (integerMemoryAddress, bool) {
	var forms [128]integerMemoryAddress
	var valid [128]bool
	for i := 0; i <= root; i++ {
		n := p.nodes[i]
		switch n.op {
		case 0x20:
			forms[i].coeff[n.a] = 1
			valid[i] = true
		case 0x41:
			forms[i].constant = n.v
			valid[i] = true
		case 0x6a, 0x6b:
			if !valid[n.a] || !valid[n.b] {
				continue
			}
			x, y := forms[n.a], forms[n.b]
			if n.op == 0x6b {
				y.constant = -y.constant
				for j := range y.coeff {
					y.coeff[j] = -y.coeff[j]
				}
			}
			x.constant += y.constant
			for j := range x.coeff {
				x.coeff[j] += y.coeff[j]
			}
			forms[i], valid[i] = x, true
		case 0x6c, 0x74:
			a, b := n.a, n.b
			if n.op == 0x6c && p.nodes[a].op == 0x41 {
				a, b = b, a
			}
			if !valid[a] || p.nodes[b].op != 0x41 {
				continue
			}
			scale := p.nodes[b].v
			if n.op == 0x74 {
				scale = uint32(1) << (scale & 31)
			}
			x := forms[a]
			x.constant *= scale
			for j := range x.coeff {
				x.coeff[j] *= scale
			}
			forms[i], valid[i] = x, true
		}
	}
	return forms[root], valid[root]
}

func (f *fn) proveIntegerMemoryLoop(p integerMemoryPlan, predicate int) (integerMemoryPlan, bool) {
	p.induction = -1
	pred := p.nodes[predicate]
	for _, edges := range [2][2]int{{pred.a, pred.b}, {pred.b, pred.a}} {
		for local := 0; local < p.locals; local++ {
			if p.written&(1<<local) == 0 || p.values[local] != edges[0] {
				continue
			}
			n := p.nodes[edges[0]]
			if n.op != 0x6a && n.op != 0x6b {
				continue
			}
			a, b := n.a, n.b
			if n.op == 0x6a && p.nodes[a].op == 0x41 {
				a, b = b, a
			}
			if a != local || p.nodes[b].op != 0x41 {
				continue
			}
			step := p.nodes[b].v
			if n.op == 0x6b {
				step = -step
			}
			if step == 0 || step >= 1<<31 || step&(step-1) != 0 {
				continue
			}
			limit, ok := p.affine(edges[1])
			if !ok {
				continue
			}
			invariant := true
			for j, c := range limit.coeff {
				if c != 0 && p.written&(1<<j) != 0 {
					invariant = false
				}
			}
			if !invariant || p.induction >= 0 {
				return p, false
			}
			p.induction, p.limit, p.step = local, edges[1], step
		}
	}
	if p.induction < 0 {
		return p, false
	}
	p.steps[p.induction] = p.step
	if p.counted {
		for local := 0; local < p.locals; local++ {
			if p.written&(1<<local) == 0 || local == p.induction {
				continue
			}
			n := p.nodes[p.values[local]]
			if n.op != 0x6a && n.op != 0x6b {
				continue
			}
			a, b := n.a, n.b
			if n.op == 0x6a && p.nodes[a].op == 0x41 {
				a, b = b, a
			}
			if a != local || p.nodes[b].op != 0x41 {
				continue
			}
			step := p.nodes[b].v
			if n.op == 0x6b {
				step = -step
			}
			p.steps[local] = step
		}
	}
	// Every load, including a dead result, must be safe before a fast path may
	// omit its original trap. Preserve its widened memory immediate separately.
	for i := 0; i < p.loadN; i++ {
		load := &p.loads[i]
		address, ok := p.affine(p.nodes[load.node].a)
		if !ok {
			return p, false
		}
		for local, c := range address.coeff {
			if c != 0 && local != p.induction && p.written&(1<<local) != 0 && (!p.mapLoads || p.steps[local] == 0) {
				return p, false
			}
			load.stride += c * p.steps[local]
		}
		load.address = address
		if load.stride >= 1<<31 {
			return p, false
		}
	}
	var valid [128]bool
	for i := 0; i < p.n; i++ {
		n := p.nodes[i]
		switch n.op {
		case 0x20:
			valid[i] = f.localType[n.a] == mtI32 && (p.steps[n.a] != 0 || p.written&(1<<n.a) == 0)
		case 0x41, 0x28:
			valid[i] = true
		case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73:
			valid[i] = valid[n.a] && valid[n.b]
		case 0x74, 0x76:
			valid[i] = valid[n.a] && (p.nodes[n.b].op == 0x41 || p.counted && valid[n.b])
		case 0x46, 0x47:
			valid[i] = p.counted && valid[n.a] && valid[n.b]
		case 0x1b:
			valid[i] = p.counted && valid[n.a] && valid[n.b] && valid[n.v]
		}
	}
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 || local == p.induction || valid[p.values[local]] {
			continue
		}
		term, count, ok := p.removeAccumulator(p.values[local], local, &valid)
		if !ok || count != 1 || term < 0 || p.reductionN == len(p.reductions) {
			return p, false
		}
		p.reductions[p.reductionN] = pureReduction{local: local, term: term}
		p.reductionN++
	}
	if p.mapStores {
		if p.storeN == 0 || p.reductionN != 0 {
			return p, false
		}
		for i := 0; i < p.storeN; i++ {
			store := &p.stores[i]
			if !valid[store.value] {
				return p, false
			}
			address, ok := p.affine(store.source)
			if !ok {
				return p, false
			}
			for local, c := range address.coeff {
				if c != 0 && p.written&(1<<local) != 0 && p.steps[local] == 0 {
					return p, false
				}
				store.stride += c * p.steps[local]
			}
			if store.stride < 4 || store.stride >= 1<<31 {
				return p, false
			}
			store.address = address
		}
		return p, true
	}
	return p, p.reductionN > 0
}

// Associative i32 addition permits one loop-carried accumulator anywhere in
// the sum DAG. Cross-accumulator and nonlinear recurrences are rejected.
func (p *integerMemoryPlan) removeAccumulator(id, local int, valid *[128]bool) (int, int, bool) {
	if valid[id] {
		return id, 0, true
	}
	if id == local {
		return -1, 1, true
	}
	n := p.nodes[id]
	if n.op != 0x6a {
		return -1, 0, false
	}
	a, na, ok := p.removeAccumulator(n.a, local, valid)
	if !ok {
		return -1, 0, false
	}
	b, nb, ok := p.removeAccumulator(n.b, local, valid)
	if !ok || na+nb > 1 {
		return -1, 0, false
	}
	if a < 0 {
		return b, na + nb, true
	}
	if b < 0 {
		return a, na + nb, true
	}
	term := p.add(pureReduceNode{op: 0x6a, a: a, b: b})
	if term < 0 {
		return -1, 0, false
	}
	valid[term] = true
	return term, na + nb, true
}
