//go:build amd64

package amd64

import "os"

var branchAddressMaximumEnabled = os.Getenv("WAGO_AMD64_BRANCH_ADDRESS_MAXIMUM") == "1"

// Capture a fact only about the current value of a direct i32 local.
func (f *fn) branchFallthroughMaximum(e *elem) (int, uint64, bool) {
	if !f.opt(optBranchAddressMaximum) || f.localBase != 0 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 || e == nil || !e.isDeferred() {
		return 0, 0, false
	}
	op := e.deferredOp()
	if op != opGtU && op != opGeU {
		return 0, 0, false
	}
	left, right := e.arg0, e.arg1
	index, ok := localAddressKey(left)
	if !ok || left.st.typ != mtI32 || right == nil || !right.isValue() || right.st.kind != stConst || right.st.typ != mtI32 {
		return 0, 0, false
	}
	maximum := uint64(uint32(right.st.cval))
	if op == opGeU {
		if maximum == 0 {
			return 0, 0, false
		}
		maximum--
	}
	return index, maximum, true
}
