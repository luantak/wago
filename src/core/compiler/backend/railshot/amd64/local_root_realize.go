//go:build amd64

package amd64

import "os"

var localRootRealizeEnabled = os.Getenv("WAGO_AMD64_LOCAL_ROOT_REALIZE") == "1"
var localRootPressureEnabled = os.Getenv("WAGO_AMD64_LOCAL_ROOT_PRESSURE") == "1"

// Count repeated reads of the local in bounded, non-trapping integer ALU
// trees. Reject memory operands and operations with implicit register needs.
func pureLocalRootReads(e *elem, x int, budget, owned *int) int {
	*budget--
	if *budget < 0 || e == nil || (e.valueType() != mtI32 && e.valueType() != mtI64) || e.st.hasGCRoot() {
		return -1
	}
	if e.isValue() {
		switch e.st.kind {
		case stLocalRef, stLocalReg:
			if e.st.idx == uint32(x) {
				return 1
			}
			return 0
		case stReg:
			*owned++
			return 0
		case stSlot, stConst:
			return 0
		}
		return -1
	}
	if !e.isDeferred() || !isBinALU(e.deferredOp()) {
		return -1
	}
	a, b := pureLocalRootReads(e.arg0, x, budget, owned), pureLocalRootReads(e.arg1, x, budget, owned)
	if a < 0 || b < 0 {
		return -1
	}
	return a + b
}

func (f *fn) realizePureLocalRoots(x int, skipFrom *elem) {
	if !f.opt(optLocalRootRealize) || f.s.pendingEffects&pendingLocalRef == 0 {
		return
	}
	for e := f.s.head.next; e != f.s.head && e != skipFrom; e = e.next {
		if !e.st.hasLogicalRoot() || !e.isDeferred() {
			continue
		}
		budget := 32
		owned := 0
		reads := pureLocalRootReads(e, x, &budget, &owned)
		if reads >= 2 && owned > 0 || f.opt(optLocalRootPressure) && reads == 1 && owned >= 2 {
			f.condense(e, regNone)
			f.stats.peep("local-root-realize")
		}
	}
}
