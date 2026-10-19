//go:build amd64

package amd64

import (
	"math/bits"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
)

var memoryMinimumBoundsEnabled = true

type localAddressMaximum struct {
	index   uint32
	maximum uint64
}

// These facts cover only the current straight-line region. They share the
// bounds-certificate invalidation boundaries, but describe integer values,
// not memory extents. A small replacement cache bounds compiler storage.
func (f *fn) rememberAddressMaximum(index int, maximum uint64) {
	limit := ^uint64(0)
	if f.localType[index] == mtI32 {
		limit = uint64(^uint32(0))
	}
	if maximum >= limit {
		return
	}
	for i := range f.localAddressMaxima {
		if f.localAddressMaximaValid&(1<<i) == 0 || f.localAddressMaxima[i].index == uint32(index) {
			f.localAddressMaxima[i] = localAddressMaximum{uint32(index), maximum}
			f.localAddressMaximaValid |= 1 << i
			return
		}
	}
	i := f.nextLocalAddressMaximum % uint8(len(f.localAddressMaxima))
	f.localAddressMaxima[i] = localAddressMaximum{uint32(index), maximum}
	f.nextLocalAddressMaximum++
}

func (f *fn) forgetAddressMaximum(index uint32) {
	for i := range f.localAddressMaxima {
		if f.localAddressMaximaValid&(1<<i) != 0 && f.localAddressMaxima[i].index == index {
			f.localAddressMaximaValid &^= 1 << i
		}
	}
}

func (f *fn) minimumMemoryAccess(memoryIndex uint32, offset uint64, size int) bool {
	if !f.opt(optMemoryMinimumBounds) {
		return false
	}
	mt, ok := f.memoryType(memoryIndex)
	if !ok || !shared.ProvenMinimumAccess(mt, 0, offset, size) {
		return false
	}
	budget := 32
	maximum := f.unsignedAddressMaximum(f.s.back(), mt.Limits.Addr64, &budget)
	return shared.ProvenMinimumAccess(mt, maximum, offset, size)
}

// The transient expression tree needs no new facts or per-local storage. A
// fixed node budget bounds recursive work; unknown expressions use the full
// unsigned domain. Address lowering still evaluates every source operation.
func (f *fn) unsignedAddressMaximum(e *elem, wide bool, budget *int) uint64 {
	limit := uint64(^uint32(0))
	typ := mtI32
	if wide {
		limit = ^uint64(0)
		typ = mtI64
	}
	if e == nil || *budget <= 0 || e.st.typ != typ {
		return limit
	}
	*budget--
	if e.elemKind() == ekValue {
		if e == f.addressValue && (e.st.kind == stReg || e.st.kind == stSlot) {
			return f.addressValueMaximum
		}
		if e.st.kind == stConst {
			value := uint64(e.st.cval)
			if !wide {
				value = uint64(uint32(value))
			}
			return value
		}
		if e.st.kind == stLocalReg || e.st.kind == stLocalRef {
			if !wide && f.loopAddressLocal == uint32(e.st.index())+1 && f.loopAddressDepth > 0 && f.loopAddressDepth <= len(f.ctrl) {
				return f.loopAddressMaximum
			}
			valid := f.localAddressMaximaValid
			if valid == 0 {
				return limit
			}
			if valid&(valid-1) == 0 {
				// The mask is nonzero, so the trailing-zero index is in [0,7].
				// The explicit mask also exposes that range to Go's bounds check.
				fact := f.localAddressMaxima[bits.TrailingZeros8(valid)&7]
				if fact.index == uint32(e.st.index()) {
					return fact.maximum
				}
				return limit
			}
			for i, fact := range f.localAddressMaxima {
				if f.localAddressMaximaValid&(1<<i) != 0 && fact.index == uint32(e.st.index()) {
					return fact.maximum
				}
			}
		}
		return limit
	}
	if e.elemKind() != ekDeferred {
		return limit
	}
	op := e.deferredOp()
	switch op {
	case opRemU:
		if f.opt(optUnsignedRemainderRange) {
			return f.unsignedRemainderMaximum(e, wide, budget, limit)
		}
		return limit
	case opAdd, opAnd, opOr, opXor, opMul, opShl, opShrU:
	default:
		return limit
	}
	left, right := e.arg0, e.arg1
	a := f.unsignedAddressMaximum(left, wide, budget)
	b := f.unsignedAddressMaximum(right, wide, budget)
	constant := uint64(0)
	known := right != nil && right.elemKind() == ekValue && right.st.kind == stConst && right.st.typ == typ
	if known {
		constant = uint64(right.st.cval)
	}
	return shared.UnsignedOperationMaximum(shared.IntOp(op), a, b, constant, known, wide)
}
