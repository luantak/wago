//go:build amd64

package amd64

// Limit flag reuse to the ordinary ALU cover. Finish a deferred left first so
// the final producer has two value leaves. ADD keeps its LEA fallback whenever
// the left is borrowed. No shift, multiply, memory operand or persistent flag
// state is admitted.
func (f *fn) tryArithmeticZeroProducer(node *elem) (Reg, bool) {
	if node.deferredOp() != opEqz {
		return regNone, false
	}
	inner := node.arg0
	if inner == nil || !inner.isDeferred() {
		return regNone, false
	}
	op := inner.deferredOp()
	if op != opAdd && op != opSub && op != opOr && op != opXor {
		return regNone, false
	}
	left, right := inner.arg0, inner.arg1
	if !right.isValue() {
		return regNone, false
	}
	switch right.st.kind {
	case stReg, stLocalReg, stGlobReg:
	case stConst:
		if !fitsImm32(right.st.cval) {
			return regNone, false
		}
	default:
		return regNone, false
	}
	if op == opAdd && left.isValue() && (left.st.kind == stLocalReg || left.st.kind == stGlobReg || left.st.kind == stConst) {
		return regNone, false
	}
	if left.isDeferred() {
		// Preserve the existing affine, scaled and associative tree covers.
		// A different XOR producer cannot take part in these outer covers.
		if left.deferredOp() != opXor || op == opXor {
			return regNone, false
		}
		f.materialize(left)
	}
	// Every remaining cover for these two value leaves ends in ADD/SUB/OR/XOR.
	// LEA forms require a borrowed left or a deferred scaled right, both excluded.
	if !left.isValue() {
		return regNone, false
	}
	r := f.condense(inner, regNone)
	f.stats.peep("arithmetic-eqz-flags")
	return r, true
}
