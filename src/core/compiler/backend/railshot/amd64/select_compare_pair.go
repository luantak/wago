//go:build amd64

package amd64

import "os"

var selectComparePairEnabled = os.Getenv("WAGO_AMD64_SELECT_COMPARE_PAIR") == "1"

// Local-copy provenance uses the existing spill-reuse tag, whose producers,
// destructive consumers, local writes and control boundaries already maintain
// its lifetime. No memory value, hash or matching instruction sequence is a key.
func (f *fn) sameSelectCompareValue(a, b *elem) bool {
	if !booleanReady(a) || !booleanReady(b) || a.st.typ != b.st.typ {
		return false
	}
	if a.st.kind == stConst || b.st.kind == stConst {
		return a.st.kind == stConst && b.st.kind == stConst && a.st.cval == b.st.cval
	}
	local := func(e *elem) (uint32, bool) {
		switch e.st.kind {
		case stLocalRef, stLocalReg:
			return e.st.idx, true
		case stReg:
			if e.st.cval > 0 && e.st.cval <= int64(len(f.locals)) {
				return uint32(e.st.cval - 1), true
			}
		}
		return 0, false
	}
	x, ok := local(a)
	y, yes := local(b)
	return ok && yes && x == y
}

// Both arms are total predicates of identical, already evaluated values. Their
// flags can be shared without suppressing a guest operation or changing traps.
func (f *fn) trySelectComparePair(cond *elem) bool {
	if !f.opt(optSelectComparePair) || !f.opt(optLocalSpillReuse) || !booleanReady(cond) || cond.st.typ != mtI32 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return false
	}
	b := cond.prev
	if b == f.s.head || !b.isDeferred() || !isCompare(b.deferredOp()) || isFloatCompare(b.deferredOp()) || b.arg1 == nil {
		return false
	}
	a := baseOfValentBlock(b).prev
	if a == f.s.head || !a.isDeferred() || !isCompare(a.deferredOp()) || isFloatCompare(a.deferredOp()) || a.arg1 == nil || a.valueType() != b.valueType() ||
		!f.sameSelectCompareValue(a.arg0, b.arg0) || !f.sameSelectCompareValue(a.arg1, b.arg1) {
		return false
	}
	ccA := condOf(a.deferredOp())
	saved := f.pinned
	cr, ownC := f.materializeRead(cond)
	f.pinned = f.pinned.add(cr)
	result := f.allocReg(0)
	f.pinned = f.pinned.add(result)
	other := f.allocReg(0)
	f.pinned = f.pinned.add(other)
	ccB := f.condenseToFlags(b)
	f.a.SetccReg(ccA, result)
	f.a.SetccReg(ccB, other)
	f.a.TestSelf(cr, false)
	f.a.Cmovcc(condE, result, other, false)
	f.pinned = saved
	if ownC {
		f.release(cr)
	}
	f.release(other)
	for e := baseOfValentBlock(a); e != a; e = e.next {
		if e.isValue() && e.st.kind == stReg {
			f.release(e.st.reg)
		}
	}
	f.consumeBlockBelow(a)
	f.erase(a)
	f.erase(cond)
	e := f.pushReg(result, mtI32)
	e.st.setValueFacts(factBoolean | factUpper32Zero)
	f.stats.peep("select-compare-pair")
	return true
}
