//go:build amd64

package amd64

import "os"

var memoryUpdateEnabled = os.Getenv("WAGO_AMD64_MEMORY_UPDATE") == "1"

// Fuse a checked, full-width read/constant-ALU/write of the same stable local
// address. The load's check already covers the store, and earlier pending reads
// have been materialized by memStore. Shared and memory64 accesses never enter
// this path. No guest memory operation or potentially trapping computation may
// occur between the read and write.
func (f *fn) tryMemoryUpdate(off uint32, size int) bool {
	if !f.opt(optMemoryUpdate) || f.guardMode || size != 4 && size != 8 || off > 0x7fffffff {
		return false
	}
	top := f.s.back()
	if top == nil || !top.isDeferred() {
		return false
	}
	var digit byte
	switch top.deferredOp() {
	case opAdd:
		digit = 0
	case opOr:
		digit = 1
	case opAnd:
		digit = 4
	case opSub:
		digit = 5
	case opXor:
		digit = 6
	default:
		return false
	}
	load, imm := top.arg0, top.arg1
	if load == nil || imm == nil || !load.isValue() || !imm.isValue() ||
		load.st.kind != stMemRef || imm.st.kind != stConst ||
		load.st.memSize() != size || load.st.memDisp() != int32(off) ||
		(size == 4 && top.valueType() != mtI32) || (size == 8 && top.valueType() != mtI64) ||
		(size == 8 && !fitsImm32(imm.st.cval)) {
		return false
	}
	addr := baseOfValentBlock(top).prev
	x, ok := localAddressKey(addr)
	if !ok || load.st.memBorrow() != x {
		return false
	}
	// No arithmetic flags survive this store; use the shorter unit-update form.
	v := int32(imm.st.cval)
	if (digit == 0 || digit == 5) && (v == 1 || v == -1) {
		f.a.IncDecIdx(RBX, load.st.reg, int32(off), (digit == 5) != (v == -1), size == 8)
	} else {
		f.a.AluImmIdx(digit, RBX, load.st.reg, int32(off), v, size == 8)
	}
	f.releaseMemRef(load.st)
	f.consumeBlockBelow(top)
	f.erase(top)
	f.erase(addr)
	f.stats.peep("memory-update")
	return true
}
