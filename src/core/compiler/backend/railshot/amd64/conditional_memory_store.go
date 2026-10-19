//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var conditionalMemoryStoreEnabled = os.Getenv("WAGO_AMD64_CONDITIONAL_MEMORY_STORE") == "1"

// A full-width read's explicit bounds check also proves a subsequent write to
// the exact same unshared address safe. Preserve the conditional write while
// avoiding a second address calculation, bounds check and branch-edge staging.
func (f *fn) tryConditionalMemoryStore(r *wasm.Reader) bool {
	if !f.opt(optConditionalMemoryStore) || f.guardMode || f.threadedMemory0 || f.memoryAddr64(0) ||
		f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 || len(f.intervalReg) != 0 ||
		f.localBase != 0 || len(f.ctrl) == 0 {
		return false
	}
	fr := &f.ctrl[len(f.ctrl)-1]
	if fr.kind != cfBlock || fr.paramN != 0 || fr.resultN != 0 || fr.branchArity() != 0 || f.depth() != fr.height+1 {
		return false
	}
	top := f.s.back()
	if top == nil || !top.isDeferred() || !isCompare(top.deferredOp()) {
		return false
	}
	old, value := top.arg1, top.arg0
	oldLeft := false
	if old == nil || !old.isValue() || old.st.kind != stMemRef {
		old, value = top.arg0, top.arg1
		oldLeft = true
	}
	valueIndex, ok := localAddressKey(value)
	if !ok || old == nil || !old.isValue() || old.st.kind != stMemRef || old.st.memBorrow() < 0 ||
		(old.st.typ != mtI32 && old.st.typ != mtI64) || old.st.typ != value.st.typ ||
		!memRefFoldable(old.st, old.st.typ == mtI64) || old.st.memDisp() < 0 {
		return false
	}
	look := *r
	label, err := look.U32()
	if err != nil || label != 0 {
		return false
	}
	getLocal := func(want uint32) bool {
		op, err := look.Byte()
		if err != nil || op != 0x20 {
			return false
		}
		index, err := look.U32()
		return err == nil && index == want
	}
	if !getLocal(uint32(old.st.memBorrow())) || !getLocal(uint32(valueIndex)) {
		return false
	}
	w := old.st.typ == mtI64
	store := byte(0x36)
	if w {
		store = 0x37
	}
	op, err := look.Byte()
	if err != nil || op != store {
		return false
	}
	align, err := look.U32()
	if err != nil || align >= 64 {
		return false
	}
	off, err := look.U32()
	if err != nil || off != uint32(old.st.memDisp()) {
		return false
	}
	if end, ok := look.Peek(); !ok || end != 0x0b {
		return false
	}

	address, disp, size := old.st.reg, old.st.memDisp(), old.st.memSize()
	cc := condOf(top.deferredOp())
	// This lease survives nested materialization, even after the deferred read
	// stops borrowing the address register. Do not let local-cache eviction
	// lend it to a comparison operand or a canonicalized operand prefix.
	savedReserved := f.reserved
	f.reserved = f.reserved.add(address)
	f.flushBelow(top)
	f.materializePendingLoads()
	f.invalidateStoreForward()
	savedPinned := f.pinned
	oldReg := f.materialize(old)
	f.pinned = f.pinned.add(oldReg)
	newReg, ownedNew := f.materializeRead(value)
	if oldLeft {
		f.cmpRR(oldReg, newReg, w)
	} else {
		f.cmpRR(newReg, oldReg, w)
	}
	over := f.a.JccPlaceholder(cc)
	f.a.StoreIdx(RBX, address, newReg, disp, size)
	f.a.PatchRel32(over, f.a.Len())
	f.release(oldReg)
	if ownedNew {
		f.release(newReg)
	}
	f.pinned = savedPinned
	f.reserved = savedReserved
	f.consumeBlockBelow(top)
	f.erase(top)
	*r = look
	f.stats.peep("conditional-memory-store")
	return true
}
