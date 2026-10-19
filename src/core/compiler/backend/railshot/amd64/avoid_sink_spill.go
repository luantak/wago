//go:build amd64

package amd64

import "os"

var avoidSinkSpillEnabled = os.Getenv("WAGO_AMD64_AVOID_SINK_SPILL") == "1"
var avoidRegionalSinkSpillEnabled = os.Getenv("WAGO_AMD64_AVOID_REGIONAL_SINK_SPILL") == "1"

func (f *fn) avoidLocalSinkSpill(e *elem, local int) {
	if !f.opt(optAvoidSinkSpill) || len(f.intervalReg) != 0 && !f.opt(optAvoidRegionalSinkSpill) || e == nil || !e.isDeferred() || !isBinALU(e.deferredOp()) || e.deferredOp().commutative() {
		return
	}
	right := e.arg1
	reg, floating, pinned := f.pinReg(local)
	if !pinned || floating || reg == RAX || reg == RDX || reg == RCX || right == nil || !right.isValue() || right.st.kind != stLocalReg || right.st.reg != reg || !treeReorderSafe(e.arg0) {
		return
	}
	// Do not reserve another register under pressure. With a genuinely free
	// budget, ordinary expression lowering preserves the borrowed local until
	// its last read, then setLocal copies the final result to the destination.
	// This avoids the store/reload forced by overwriting that destination while
	// evaluating the left subtree. No expression identity or corpus is matched.
	blocked := f.pinned.union(f.pinnedLocalMask).union(f.reserved).add(reg)
	free := int16(0)
	for _, r := range gpAlloc {
		if !blocked.has(r) && f.regUser[r] == nil {
			free++
		}
	}
	if free < treeRegisterNeed(e) {
		return
	}
	f.condense(e, regNone)
	f.stats.peep("avoid-sink-spill")
}
