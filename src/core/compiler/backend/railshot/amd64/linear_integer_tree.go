//go:build amd64

package amd64

import "os"

var linearIntegerTreeEnabled = os.Getenv("WAGO_AMD64_LINEAR_INTEGER_COLLECT") == "1"

type linearIntegerTerm struct {
	source      *elem
	coefficient uint32
	index       uint32
	reads       int
	reg         Reg
	owned       bool
}
type linearIntegerTree struct {
	f                         *fn
	terms                     [8]linearIntegerTerm
	count, budget, operations int
	constant                  uint32
}

// Arithmetic is collected modulo 2^32. Borrowed current-local reads have no
// observable effects; captured older reads, globals, memory, and trapping nodes
// fall back. Fixed scratch and a node budget bound planning cost.
func (w *linearIntegerTree) collect(e *elem, scale uint32) bool {
	if e == nil || w.budget == 0 || e.valueType() != mtI32 {
		return false
	}
	w.budget--
	if e.elemKind() == ekValue {
		if e.st.kind == stConst {
			w.constant += uint32(e.st.cval) * scale
			return true
		}
		if e.st.kind != stLocalReg && e.st.kind != stLocalRef {
			return false
		}
		for i := 0; i < w.count; i++ {
			t := &w.terms[i]
			if t.index == e.st.idx {
				t.coefficient += scale
				t.reads++
				return true
			}
		}
		if w.count == len(w.terms) {
			return false
		}
		w.terms[w.count] = linearIntegerTerm{source: e, coefficient: scale, index: e.st.idx, reads: 1}
		w.count++
		return true
	}
	if !e.isDeferred() {
		return false
	}
	a, b := e.arg0, e.arg1
	w.operations++
	switch e.deferredOp() {
	case opAdd:
		return w.collect(a, scale) && w.collect(b, scale)
	case opSub:
		return w.collect(a, scale) && w.collect(b, -scale)
	case opMul:
		if a != nil && a.elemKind() == ekValue && a.st.kind == stConst {
			a, b = b, a
		}
		if b == nil || b.elemKind() != ekValue || b.st.kind != stConst {
			return false
		}
		return w.collect(a, scale*uint32(b.st.cval))
	case opShl:
		if b == nil || b.elemKind() != ekValue || b.st.kind != stConst {
			return false
		}
		return w.collect(a, scale<<(uint32(b.st.cval)&31))
	}
	return false
}

func (f *fn) tryLinearIntegerTree(node *elem, dest Reg) Reg {
	if !f.opt(optLinearIntegerTree) || !f.hasLoop || node.valueType() != mtI32 || node.deferredDepth() < 4 || (node.deferredOp() != opAdd && node.deferredOp() != opSub) || len(f.customInstructions) != 0 {
		return regNone
	}
	w := linearIntegerTree{f: f, budget: 64}
	if !w.collect(node, 1) {
		return regNone
	}
	nonzero, duplicate := 0, false
	for _, t := range w.terms[:w.count] {
		if t.coefficient != 0 {
			nonzero++
		}
		duplicate = duplicate || t.reads > 1
	}
	// Require repeated sources and at least three eliminated arithmetic nodes.
	if !duplicate || w.operations < nonzero*2+3 {
		return regNone
	}
	// Keep every surviving borrowed source live while materializing stack locals.
	// Reserve room for those loads plus the result and coefficient before emitting
	// anything: unlike the original tree, this cover holds all sources at once.
	blocked := f.pinned.union(f.pinnedLocalMask).union(f.reserved)
	needed := 2
	for _, t := range w.terms[:w.count] {
		if t.coefficient == 0 {
			continue
		}
		if t.source.st.kind == stLocalReg {
			blocked = blocked.add(t.source.st.reg)
		} else {
			needed++
		}
	}
	available := 0
	for _, r := range gpAlloc {
		if !blocked.has(r) && f.regUser[r] == nil {
			available++
		}
	}
	if available < needed {
		return regNone
	}
	savedPins := f.pinned
	for i := 0; i < w.count; i++ {
		t := &w.terms[i]
		if t.coefficient == 0 {
			continue
		}
		t.reg, t.owned = f.materializeRead(t.source)
		f.pinned = f.pinned.add(t.reg)
	}
	result := dest
	for _, t := range w.terms[:w.count] {
		if t.coefficient != 0 && t.reg == result {
			result = regNone
			break
		}
	}
	if result == regNone {
		result = f.allocReg(0)
	}
	f.a.MovImm32(result, int32(w.constant))
	coefficient := regNone
	for _, t := range w.terms[:w.count] {
		c := t.coefficient
		if c == 0 {
			continue
		}
		switch {
		case c == 1:
			f.a.AluRR(0x01, result, t.reg, false)
		case c == ^uint32(0):
			f.a.AluRR(0x29, result, t.reg, false)
		default:
			if coefficient == regNone {
				coefficient = f.allocReg(maskOf(result))
			}
			f.a.ImulRRI(coefficient, t.reg, int32(c), false)
			f.a.AluRR(0x01, result, coefficient, false)
		}
	}
	if coefficient != regNone {
		f.release(coefficient)
	}
	if dest != regNone && result != dest {
		f.moveInt(dest, result, mtI32)
		f.release(result)
		result = dest
	}
	f.pinned = savedPins
	for _, t := range w.terms[:w.count] {
		if t.coefficient != 0 && t.owned && t.reg != result {
			f.release(t.reg)
		}
	}
	f.stats.peep("linear-integer-tree")
	f.consumeBlockBelow(node)
	f.occupy(node, result)
	node.setDeferredOp(opNone)
	return result
}
