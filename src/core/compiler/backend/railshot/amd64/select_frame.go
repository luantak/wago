//go:build amd64

package amd64

import "os"

var selectFrameEnabled = os.Getenv("WAGO_AMD64_SELECT_FRAME") == "1"

func (f *fn) selectFrameOffset(e *elem) (int32, bool) {
	if e == nil || !e.isValue() || (e.st.typ != mtI32 && e.st.typ != mtI64) || e.st.hasGCRoot() {
		return 0, false
	}
	switch e.st.kind {
	case stLocalRef:
		return f.localAddr(e.st.index()), true
	case stSlot:
		return f.spillOff(e.st.slotIndex()), true
	default:
		return 0, false
	}
}

// A frame-backed alternative is safe to read unconditionally, unlike a guest
// memory operand. Keep its stack owner linked through both materializations so
// allocator spills cannot reuse its slot before CMOV consumes it.
func (f *fn) trySelectFrame(cond *elem) bool {
	if !f.opt(optSelectFrame) || cond == nil || !cond.isValue() {
		return false
	}
	b := cond.prev
	if _, ok := f.selectFrameOffset(b); !ok {
		return false
	}
	a := b.prev
	// A ready constant arm favors loading the alternative independently of the
	// conditional move. Preserve that scheduling freedom for constant selects.
	if a == f.s.head || !a.isValue() || a.st.typ != b.st.typ || a.st.hasGCRoot() || a.st.kind == stMemRef || a.st.kind == stConst {
		return false
	}
	ar := f.materialize(a)
	f.pinned = f.pinned.add(ar)
	cr := f.materialize(cond)
	disp, ok := f.selectFrameOffset(b)
	if !ok {
		f.pinned = f.pinned.remove(ar)
		return false
	}
	if regallocCheckEnabled {
		f.checkUse(b)
	}
	f.a.TestSelf(cr, false)
	f.a.CmovccDisp(condE, ar, RSP, disp, a.st.typ == mtI64)
	typ := a.st.typ
	f.pinned = f.pinned.remove(ar)
	f.release(cr)
	f.erase(cond)
	f.erase(b)
	f.erase(a)
	f.pushReg(ar, typ)
	f.stats.peep("select-frame")
	return true
}
