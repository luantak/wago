//go:build arm64

package arm64

// A SUB/XOR result consumed only by eqz is dead. Comparing its concrete
// operands preserves modular equality without a destructive arithmetic result.
// Deferred operands keep the existing tree covers and evaluation order.
func (f *fn) foldArithmeticEqzCompare(node *elem) {
	if node.deferredOp() != opEqz {
		return
	}
	inner := f.s.arg0(node)
	if inner == nil || !inner.isDeferred() || (inner.deferredOp() != opSub && inner.deferredOp() != opXor) {
		return
	}
	left, right := f.s.arg0(inner), f.s.arg1(inner)
	leaf := func(e *elem) bool {
		if e == nil || e.elemKind() != ekValue {
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
	node.setChildren(inner.child0ID(), inner.child1ID())
	node.setDeferredOp(opEq)
	node.setDeferredDepth(1)
	f.erase(inner)
	f.stats.peep("arithmetic-eqz-compare")
}
