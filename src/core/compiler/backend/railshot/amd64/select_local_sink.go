//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var selectLocalSinkEnabled = os.Getenv("WAGO_AMD64_SELECT_LOCAL_SINK") == "1"

// A select immediately assigned back to its false-arm local can update that
// local with CMOVNE. No deferred branch work or guest memory is moved across
// the assignment, and older references are realized before the register write.
func (f *fn) trySelectLocalSink(r *wasm.Reader) bool {
	if !f.opt(optSelectLocalSink) || r == nil {
		return false
	}
	look := *r
	op, err := look.Byte()
	if err != nil || op != 0x21 {
		return false
	}
	i, err := look.U32()
	if err != nil || uint64(i)+uint64(f.localBase) >= uint64(f.nLocals) {
		return false
	}
	x := int(i) + f.localBase
	cond := f.s.back()
	if cond == nil || !cond.isValue() || cond.st.typ != mtI32 || cond.st.kind == stMemRef {
		return false
	}
	b := cond.prev
	if b == f.s.head || !b.isValue() || b.st.kind != stLocalReg || b.st.index() != x || b.st.hasGCRoot() {
		return false
	}
	a := b.prev
	if a == f.s.head || !a.isValue() || (a.st.typ != mtI32 && a.st.typ != mtI64) || a.st.typ != b.st.typ || a.st.kind == stMemRef || a.st.hasGCRoot() {
		return false
	}
	f.realizeLocalRefs(x, a)
	pr, floating, ok := f.pinReg(x)
	if !ok || floating || b.st.kind != stLocalReg || b.st.reg != pr {
		return false
	}
	savedReserved, savedPinned := f.reserved, f.pinned
	f.reserved = f.reserved.add(pr)
	ar, aOwned := f.materializeRead(a)
	f.pinned = f.pinned.add(ar)
	cr, cOwned := f.materializeRead(cond)
	f.forgetLocalSpillCopies(x)
	f.invalidateBoundsCertFor(1, uint32(x))
	f.forgetAddressMaximum(uint32(x))
	f.invalidateGCLoadFactsForLocal(x)
	f.a.TestSelf(cr, false)
	f.a.Cmovcc(condNE, pr, ar, a.st.typ == mtI64)
	f.markLocalDirty(x)
	f.reserved, f.pinned = savedReserved, savedPinned
	if aOwned {
		f.release(ar)
	}
	if cOwned {
		f.release(cr)
	}
	typ := a.st.typ
	f.erase(cond)
	f.erase(b)
	f.erase(a)
	f.pushValue(storage{kind: stLocalReg, typ: typ, reg: pr, idx: uint32(x)})
	f.stats.peep("select-local-sink")
	return true
}
