//go:build amd64

package amd64

import "os"

var localSpillReuseEnabled = os.Getenv("WAGO_AMD64_LOCAL_SPILL_REUSE") == "1"

// Integer register values may carry local-index+1 in the otherwise unused cval
// field after an unpinned tee stores their exact value. Mutable consumers clear
// it before emission; local writes and control boundaries invalidate it too.
func (f *fn) forgetLocalSpillCopies(x int) {
	if !f.opt(optLocalSpillReuse) {
		return
	}
	for _, e := range f.regUser {
		if e != nil && e.isValue() && e.st.kind == stReg && !e.st.typ.isXMM() &&
			(x < 0 || e.st.cval == int64(x+1)) {
			e.st.cval = 0
		}
	}
}

func (f *fn) reuseLocalSpill(e *elem) bool {
	if !f.opt(optLocalSpillReuse) || !e.isValue() || e.st.kind != stReg ||
		(e.st.typ != mtI32 && e.st.typ != mtI64) || e.st.hasGCRoot() || e.st.cval <= 0 {
		return false
	}
	x := int(e.st.cval - 1)
	if x >= len(f.locals) || f.localType[x] != e.st.typ ||
		(f.locals[x].state != lsMem && f.locals[x].state != lsStackReg) {
		return false
	}
	f.regUser[e.st.reg] = nil
	f.replaceStorage(e, storage{kind: stLocalRef, typ: e.st.typ, idx: uint32(x)})
	f.stats.peep("local-spill-reuse")
	return true
}
