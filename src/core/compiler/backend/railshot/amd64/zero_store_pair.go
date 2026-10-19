//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var zeroStorePairEnabled = os.Getenv("WAGO_AMD64_ZERO_STORE_PAIR") == "1"

func (f *fn) tryZeroStorePair(r *wasm.Reader, off uint32, size int) bool {
	if !f.opt(optZeroStorePair) || !f.inLoop() || f.guardMode || size != 8 || off < 24 || off > 0x7ffffff7 {
		return false
	}
	top := f.s.back()
	if top == nil || !top.isValue() || top.st.kind != stConst || top.st.typ != mtI64 || top.st.cval != 0 {
		return false
	}
	x, ok := localAddressKey(top.prev)
	if !ok {
		return false
	}
	look := *r
	for i := uint32(1); i < 4; i++ {
		op, err := look.Byte()
		if err != nil || op != 0x20 {
			return false
		}
		idx, err := look.U32()
		if err != nil || uint64(idx)+uint64(f.localBase) != uint64(x) {
			return false
		}
		op, err = look.Byte()
		if err != nil || op != 0x42 {
			return false
		}
		v, err := look.I64()
		if err != nil || v != 0 {
			return false
		}
		op, err = look.Byte()
		if err != nil || op != 0x37 {
			return false
		}
		align, err := look.U32()
		if err != nil || align >= 64 {
			return false
		}
		next, err := look.U32()
		if err != nil || next != off-8*i {
			return false
		}
	}
	// Four scalar stores amortize one zero-vector setup over two vector stores.
	// The high-address store runs first. Its check covers the entire run,
	// so merging cannot turn a later trap into loss of an earlier write.
	f.erase(top)
	ea, owned, _, disp := f.memAddr(off, 8, true, 0)
	f.pinned = f.pinned.add(ea)
	zero := f.allocFReg(0)
	opVPxor.emit(f, zero, zero, zero)
	f.mov128StoreIdx(RBX, ea, zero, disp-8)
	f.mov128StoreIdx(RBX, ea, zero, disp-24)
	f.releaseF(zero)
	f.pinned = f.pinned.remove(ea)
	if owned {
		f.release(ea)
	}
	*r = look
	f.stats.peep("zero-store-pair")
	return true
}
