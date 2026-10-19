//go:build amd64

package amd64

import "os"

var lonePredicateEnabled = os.Getenv("WAGO_AMD64_LONE_PREDICATE") != "0"

// Older operands require full staging: canonical destinations may overwrite
// their existing spill homes. With one owned numeric root outside the local pin
// pool, no prefix exists and local convergence cannot restore its register.
// It remains tracked and spillable until popBranchCondition after convergence.
func (f *fn) flushBranchPredicate(predicate *elem, loopBackedge bool) {
	// Admit borrowed values only at latches, removing repeated stack traffic.
	// Other control entries retain their established staging/layout policy.
	if lonePredicateEnabled && loopBackedge && f.depth() == 1 && predicate.isValue() && predicate.st.typ == mtI32 && !predicate.st.hasGCRoot() {
		if predicate.st.kind == stLocalReg && f.pinnedLocalMask.has(predicate.st.reg) &&
			!f.usesCalls && !f.pinRelinquished && !f.lazyZero {
			// With no prefix and no local-home convergence, the permanent
			// pin survives unchanged until popBranchCondition's read-only test.
			f.flushBelow(predicate)
			f.stats.peep("lone-local-predicate")
			f.stats.peep("lone-local-predicate-borrow")
			return
		}
		switch predicate.st.kind {
		case stLocalReg, stLocalRef, stGlobReg:
			// Own the value before convergence can change a borrowed home.
			// The tracked register may still spill if reconciliation needs it;
			// normally a single copy/load replaces a stack store and reload.
			f.materialize(predicate)
			f.stats.peep("lone-local-predicate")
		}
	}
	if lonePredicateEnabled && f.depth() == 1 && predicate.isValue() && predicate.st.kind == stReg && predicate.st.typ == mtI32 && !f.pinnedLocalMask.has(predicate.st.reg) && !predicate.st.hasGCRoot() {
		f.flushBelow(predicate)
		f.stats.peep("lone-predicate")
		return
	}
	f.flush()
}
