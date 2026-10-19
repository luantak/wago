//go:build amd64

package amd64

import "github.com/wago-org/wago/src/core/compiler/wasm"

// This is an analysis-only plan. Erasing a repeated store is not authorized
// until runtime range, nonwrapping-trip and load/store disjointness guards pass.
type invariantStorePlan struct {
	p             integerMemoryPlan
	stores        [8]int
	backedge, end int
}

func (f *fn) inspectInvariantStoreLoop(source *wasm.Reader) (out invariantStorePlan, ok bool) {
	if f.nLocals > 32 || len(f.localType) != f.nLocals {
		return out, false
	}
	for _, typ := range f.localType {
		if typ != mtI32 {
			return out, false
		}
	}
	p := integerMemoryPlan{locals: f.nLocals, n: f.nLocals, induction: -1}
	for i := 0; i < p.locals; i++ {
		p.nodes[i] = pureReduceNode{op: 0x20, a: i}
		p.values[i] = i
	}
	var stack [96]int
	depth := 0
	scan := *source
	start := scan.Offset()
	for steps := 0; steps < 384 && scan.Offset()-start < 1024; steps++ {
		at := scan.Offset() - start
		op, err := scan.Byte()
		if err != nil {
			return out, false
		}
		look := scan
		var imm wasm.InstructionImmediate
		if f.classifier.ClassifyInto(&scan, op, &imm) != nil {
			return out, false
		}
		id := -1
		switch op {
		case 0x20, 0x21, 0x22:
			x := int(imm.Index)
			if x >= p.locals {
				return out, false
			}
			if op == 0x20 {
				id = p.values[x]
			} else {
				if depth == 0 {
					return out, false
				}
				p.values[x] = stack[depth-1]
				p.written |= 1 << x
				if op == 0x21 {
					depth--
				}
				continue
			}
		case 0x41:
			v, err := look.I32()
			if err != nil {
				return out, false
			}
			id = p.add(pureReduceNode{op: op, v: uint32(v)})
		case 0x45, 0x67, 0x68, 0x69:
			if depth < 1 {
				return out, false
			}
			depth--
			id = p.add(pureReduceNode{op: op, a: stack[depth]})
		case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f,
			0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78:
			if depth < 2 {
				return out, false
			}
			depth -= 2
			id = p.add(pureReduceNode{op: op, a: stack[depth], b: stack[depth+1]})
		case 0x1a:
			if depth == 0 {
				return out, false
			}
			depth--
			continue
		case 0x1b:
			if depth < 3 {
				return out, false
			}
			depth -= 3
			id = p.add(pureReduceNode{op: op, a: stack[depth], b: stack[depth+1], v: uint32(stack[depth+2])})
		case 0x28:
			if depth == 0 || p.loadN == len(p.loads) || imm.MemIndex != 0 || imm.MemOffset > 0xffffffff {
				return out, false
			}
			depth--
			id = p.add(pureReduceNode{op: op, a: stack[depth]})
			if id < 0 {
				return out, false
			}
			p.loads[p.loadN] = integerMemoryLoad{node: id, offset: imm.MemOffset}
			p.loadN++
		case 0x36:
			if depth < 2 || p.storeN == len(p.stores) || imm.MemIndex != 0 || imm.MemOffset > 0xffffffff {
				return out, false
			}
			depth -= 2
			p.stores[p.storeN] = integerMemoryStore{source: stack[depth], value: stack[depth+1], offset: imm.MemOffset}
			out.stores[p.storeN] = at
			p.storeN++
			continue
		case 0x0d:
			if imm.Index != 0 || depth != 1 || p.storeN == 0 || p.loadN == 0 || p.nodes[stack[0]].op != 0x47 {
				return out, false
			}
			end, err := scan.Byte()
			if err != nil || end != 0x0b {
				return out, false
			}
			out.backedge, out.end = at, scan.Offset()-start
			return proveInvariantStoreLoop(p, stack[0], out)
		default:
			// No nested control, external effects, trapping arithmetic, narrow
			// memory operations or data-dependent addressing enters this proof.
			return out, false
		}
		if id < 0 || depth == len(stack) {
			return out, false
		}
		stack[depth] = id
		depth++
	}
	return out, false
}

func proveInvariantStoreLoop(p integerMemoryPlan, predicate int, out invariantStorePlan) (invariantStorePlan, bool) {
	var known [32]bool
	for local := 0; local < p.locals; local++ {
		if p.written&(1<<local) == 0 {
			known[local] = true
			continue
		}
		form, ok := p.affine(p.values[local])
		if !ok || form.coeff[local] != 1 {
			continue
		}
		valid := true
		for j, c := range form.coeff {
			if j != local && c != 0 {
				valid = false
			}
		}
		if valid {
			known[local] = true
			p.steps[local] = form.constant
		}
	}
	stride := func(a integerMemoryAddress) (uint32, bool) {
		var step uint32
		for local, c := range a.coeff {
			if c != 0 {
				if !known[local] {
					return 0, false
				}
				step += c * p.steps[local]
			}
		}
		return step, step < 1<<31
	}
	pred := p.nodes[predicate]
	for _, edge := range [2][2]int{{pred.a, pred.b}, {pred.b, pred.a}} {
		for local := 0; local < p.locals; local++ {
			step := p.steps[local]
			if !known[local] || step == 0 || step >= 1<<31 || step&(step-1) != 0 || p.values[local] != edge[0] {
				continue
			}
			limit, ok := p.affine(edge[1])
			if !ok {
				continue
			}
			invariant := true
			for j, c := range limit.coeff {
				if c != 0 && (!known[j] || p.steps[j] != 0) {
					invariant = false
				}
			}
			if !invariant {
				continue
			}
			if p.induction >= 0 {
				return out, false
			}
			p.induction, p.limit, p.step = local, edge[1], step
		}
	}
	if p.induction < 0 {
		return out, false
	}
	for i := 0; i < p.storeN; i++ {
		s := &p.stores[i]
		address, ok := p.affine(s.source)
		if !ok {
			return out, false
		}
		step, ok := stride(address)
		if !ok || step != 0 {
			return out, false
		}
		s.address = address
		if i > 0 && (address != p.stores[0].address || s.offset != p.stores[0].offset) {
			return out, false
		}
	}
	for i := 0; i < p.loadN; i++ {
		l := &p.loads[i]
		address, ok := p.affine(p.nodes[l.node].a)
		if !ok {
			return out, false
		}
		step, ok := stride(address)
		if !ok {
			return out, false
		}
		// A definitely identical source must observe the updated value. A later
		// general memory-to-register pass can handle it; this store-sinking plan
		// requires every source range disjoint from the destination at runtime.
		if address == p.stores[0].address && l.offset == p.stores[0].offset {
			return out, false
		}
		l.address, l.stride = address, step
	}
	out.p = p
	return out, true
}
