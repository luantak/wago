//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var byteStoreRunEnabled = os.Getenv("WAGO_AMD64_BYTE_STORE_RUN") == "1"
var byteStoreMovzxEnabled = os.Getenv("WAGO_AMD64_BYTE_STORE_MOVZX") == "1"

func (f *fn) tryByteStoreRun(r *wasm.Reader, off uint32, size int) bool {
	// Regional locals require their exact get-event positions. Keep their
	// normal lowering until the scanner can preserve those events.
	if !f.opt(optByteStoreRun) || f.guardMode || f.intervalControl || f.intervalRegLimit != 0 || size != 1 || off < 3 || off > 0x7fffffff {
		return false
	}
	top := f.s.back()
	if top == nil || !top.isValue() || top.st.typ != mtI32 || (top.st.kind != stLocalReg && top.st.kind != stLocalRef) {
		return false
	}
	x, ok := localAddressKey(top.prev)
	if !ok {
		return false
	}
	look := *r
	var values [3]wasm.Reader
	for i := uint32(0); i < 3; i++ {
		op, err := look.Byte()
		if err != nil || op != 0x20 {
			return false
		}
		index, err := look.U32()
		if err != nil || uint64(index)+uint64(f.localBase) != uint64(x) {
			return false
		}
		op, err = look.Byte()
		if err != nil || op != 0x20 {
			return false
		}
		values[i] = look
		index, err = look.U32()
		local := uint64(index) + uint64(f.localBase)
		if err != nil || local >= uint64(len(f.localType)) || f.localType[local] != mtI32 {
			return false
		}
		op, err = look.Byte()
		if err != nil || op != 0x3a {
			return false
		}
		align, err := look.U32()
		if err != nil || align >= 64 {
			return false
		}
		next, err := look.U32()
		if err != nil || next != off-i-1 {
			return false
		}
	}
	// The first (highest-address) byte traps before any write if any part of
	// the merged word is out of bounds. The skipped operations are local gets,
	// so cannot trap, mutate memory, or change the common address.
	packed := f.materialize(f.popValue())
	saved := f.pinned
	f.pinned = f.pinned.add(packed)
	f.a.ShiftImm(4, packed, 24, false)
	for i := range values {
		if err := f.emitPlain(&values[i], 0x20); err != nil {
			panic(err)
		}
		value := f.popValue()
		var part Reg
		if f.opt(optByteStoreMovzx) {
			src, owned := f.materializeRead(value)
			part = src
			if !owned {
				pins := f.pinned
				f.pinned = f.pinned.add(src)
				part = f.allocReg(maskOf(src))
				f.pinned = pins
			}
			f.a.Movzx8(part, src, false)
			f.stats.peep("byte-store-movzx")
		} else {
			part = f.materialize(value)
			f.a.AluRI(4, part, 255, false)
		}
		if i != 2 {
			f.a.ShiftImm(4, part, byte(16-i*8), false)
		}
		f.a.AluRR(0x09, packed, part, false)
		f.release(part)
	}
	f.invalidateStoreForward()
	ea, owned, _, disp := f.memAddr(off, 1, true, 0)
	f.a.StoreIdx(RBX, ea, packed, disp-3, 4)
	if owned {
		f.release(ea)
	}
	f.pinned = saved
	f.release(packed)
	*r = look
	f.stats.peep("byte-store-run")
	return true
}
