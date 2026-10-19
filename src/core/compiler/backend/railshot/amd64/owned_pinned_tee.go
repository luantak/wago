//go:build amd64

package amd64

import "os"

var ownedPinnedTeeEnabled = os.Getenv("WAGO_AMD64_OWNED_PINNED_TEE") == "1"

// A ready owned value remains independent after assigning its value to a
// pinned local. Retaining that existing copy avoids recreating it if the local
// is overwritten while the tee result is still on the operand stack.
func (f *fn) keepOwnedPinnedTee(e *elem, x int, dest Reg, tee bool) bool {
	if !tee || !f.opt(optOwnedPinnedTee) || f.moduleEH || len(f.customInstructions) != 0 ||
		e == nil || !e.isValue() || e.st.kind != stReg || e.st.reg == dest ||
		(e.st.typ != mtI32 && e.st.typ != mtI64) || e.st.hasGCRoot() || f.gcFrameLocal(x) ||
		f.regUser[e.st.reg] != e {
		return false
	}
	if regallocCheckEnabled {
		f.checkUse(e)
	}
	f.moveInt(dest, e.st.reg, e.st.typ)
	if regallocCheckEnabled {
		f.checkOccupy(e, dest, false)
	}
	f.markLocalDirty(x)
	e.st.cval = 0
	if f.opt(optSelectComparePair) && f.opt(optLocalSpillReuse) {
		e.st.cval = int64(x + 1)
	}
	f.stats.peep("owned-pinned-tee")
	return true
}
