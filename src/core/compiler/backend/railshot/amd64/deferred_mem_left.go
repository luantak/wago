//go:build amd64

package amd64

import "os"

var deferredMemLeftEnabled = os.Getenv("WAGO_AMD64_DEFERRED_MEM_LEFT") == "1"

// Finish the existing right-first evaluation before selecting the commutative
// r/m form. With explicit checks a pending scalar load cannot trap here. No
// calls, writes, div/rem, or position-dependent regional local leases move.
func (f *fn) prepareDeferredMemoryLeft(node *elem) {
	if !f.opt(optDeferredMemLeft) || !commuteMemLeftEnabled || len(f.intervalReg) != 0 ||
		f.guardMode || f.threadedMemory0 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 ||
		!node.deferredOp().commutative() || (node.valueType() != mtI32 && node.valueType() != mtI64) {
		return
	}
	left, right := node.arg0, node.arg1
	if !left.isValue() || left.st.kind != stMemRef ||
		!right.isDeferred() || !checkedMemoryTreeSafe(right) {
		return
	}
	if left.st.kind == stMemRef && !memRefFoldable(left.st, node.valueType() == mtI64) {
		return
	}
	f.materialize(right)
	f.stats.peep("deferred-mem-left")
}

func checkedMemoryTreeSafe(e *elem) bool {
	if e == nil {
		return false
	}
	if e.isValue() {
		return e.st.kind == stMemRef || treeReorderSafe(e)
	}
	if !e.isDeferred() || !(isBinALU(e.deferredOp()) || isShift(e.deferredOp()) || isCompare(e.deferredOp()) || e.deferredOp() == opEqz || isUnary(e.deferredOp()) || isConvert(e.deferredOp())) {
		return false
	}
	return checkedMemoryTreeSafe(e.arg0) && (e.arg1 == nil || checkedMemoryTreeSafe(e.arg1))
}
