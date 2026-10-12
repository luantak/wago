//go:build amd64

package amd64

// A SUB/XOR result consumed only by eqz is dead. Comparing its concrete
// operands preserves modular equality without a destructive arithmetic result.
// Deferred operands keep the existing tree covers and evaluation order.
func (f *fn) foldArithmeticEqzCompare(node *elem) {
	if node.deferredOp() != opEqz {
		return
	}
	inner := node.arg0
	if inner == nil || !inner.isDeferred() || (inner.deferredOp() != opSub && inner.deferredOp() != opXor) {
		return
	}
	left, right := inner.arg0, inner.arg1
	leaf := func(e *elem) bool {
		if e == nil || !e.isValue() {
			return false
		}
		switch e.st.kind {
		case stReg, stLocalReg, stGlobReg, stConst:
			return true
		}
		return false
	}
	if !leaf(left) || !leaf(right) {
		return
	}
	node.arg0, node.arg1 = left, right
	node.setDeferredOp(opEq)
	node.setDeferredDepth(1)
	f.erase(inner)
	f.stats.peep("arithmetic-eqz-compare")
}
