//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var selectFlagsSinkEnabled = os.Getenv("WAGO_AMD64_SELECT_FLAGS_SINK") == "1"

// A plain true arm already in the destination local needs no temporary result.
// Prepare the other arm, consume the comparison, then update the destination.
// Earlier references retain the old value; no deferred guest work is reordered.
func (f *fn) trySelectFlagsSink(r *wasm.Reader) bool {
	if !f.opt(optSelectFlagsSink) || !f.opt(optSelectLocalSink) {
		return false
	}
	look := *r
	op, err := look.Byte()
	if err != nil || op != 0x21 && op != 0x22 {
		return false
	}
	i, err := look.U32()
	if err != nil || uint64(i)+uint64(f.localBase) >= uint64(f.nLocals) {
		return false
	}
	x := int(i) + f.localBase
	cond := f.s.back()
	if !isFusableCompare(cond) || isFloatCompare(cond.deferredOp()) {
		return false
	}
	plain := func(e *elem) bool {
		return e != nil && e != f.s.head && e.isValue() && (e.st.typ == mtI32 || e.st.typ == mtI64) && e.st.kind != stMemRef && !e.st.hasGCRoot()
	}
	if !plain(cond.arg0) || cond.arg1 != nil && !plain(cond.arg1) {
		return false
	}
	b := baseOfValentBlock(cond).prev
	if !plain(b) {
		return false
	}
	a := b.prev
	if !plain(a) || a.st.typ != b.st.typ || a.st.kind != stLocalReg || a.st.index() != x {
		return false
	}
	f.realizeLocalRefs(x, a)
	pr, floating, ok := f.pinReg(x)
	if !ok || floating || a.st.kind != stLocalReg || a.st.reg != pr || f.pinRelinquished && f.regUser[pr] != nil {
		return false
	}
	savedPinned, savedReserved := f.pinned, f.reserved
	f.reserved = f.reserved.add(pr)
	br, owned := f.materializeSelectReadBranch(b, b.st.typ)
	f.pinned = f.pinned.add(br)
	if !owned {
		f.reserved = f.reserved.add(br)
	}
	typ := a.st.typ
	cc := f.condenseToFlags(cond)
	f.a.Cmovcc(invertCond(cc), pr, br, typ == mtI64)
	f.forgetLocalSpillCopies(x)
	f.invalidateBoundsCertFor(1, uint32(x))
	f.forgetAddressMaximum(uint32(x))
	f.invalidateGCLoadFactsForLocal(x)
	f.markLocalDirty(x)
	f.pinned, f.reserved = savedPinned, savedReserved
	if owned {
		f.release(br)
	}
	f.erase(b)
	f.erase(a)
	f.pushValue(storage{kind: stLocalReg, typ: typ, reg: pr, idx: uint32(x)})
	f.stats.peep("select-flags-sink")
	return true
}
