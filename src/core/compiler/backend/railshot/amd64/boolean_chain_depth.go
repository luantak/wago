//go:build amd64

package amd64

import "os"

var booleanChainDepthEnabled = os.Getenv("WAGO_AMD64_BOOLEAN_CHAIN_DEPTH") == "1"

// Only flat reductions of total comparisons over evaluated values may exceed
// the ordinary arithmetic depth cap. The chain emitter uses fixed scratch,
// independently of the number of leaves; guest loads and divisions cannot enter.
func (f *fn) extendBooleanChain(op wOp, typ machineType, left, right *elem) bool {
	if !f.opt(optBooleanChainDepth) || !f.opt(optBooleanChain) || typ != mtI32 || (op != opAnd && op != opOr) || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return false
	}
	var leaves [16]*elem
	n, budget := 0, 64
	return booleanLeaves(left, op, &leaves, &n, &budget) && booleanLeaves(right, op, &leaves, &n, &budget)
}
