//go:build amd64

package amd64

import (
	"math/rand"
	"testing"
)

func TestUnsignedRemainderMaximum(t *testing.T) {
	for _, wide := range []bool{false, true} {
		typ, limit := mtI32, uint64(^uint32(0))
		if wide {
			typ, limit = mtI64, ^uint64(0)
		}
		for _, op := range []wOp{opRemU} {
			for _, divisor := range []uint64{0, 1, 2, 3, 7, 1000, 65537, 0x80000000, 0xffffffff, 1 << 63, ^uint64(0)} {
				f := fn{policy: currentCodegenPolicy(), s: newStack()}
				left := f.pushValue(storage{kind: stLocalRef, typ: typ, idx: 0})
				right := f.pushValue(storage{kind: stConst, typ: typ, cval: int64(divisor)})
				e := f.s.alloc()
				e.setElemKind(ekDeferred)
				e.setValueType(typ)
				e.setDeferredOp(op)
				e.arg0, e.arg1 = left, right
				d := divisor & limit
				for _, maximum := range []uint64{0, 1, 255, 65535, limit} {
					f.localAddressMaxima[0] = localAddressMaximum{0, maximum}
					f.localAddressMaximaValid = 1
					budget := 32
					bound := f.unsignedRemainderMaximum(e, wide, &budget, limit)
					want := limit
					if d != 0 {
						want = maximum / d
						if op == opRemU {
							want = min(maximum, d-1)
						}
					}
					if bound != want {
						t.Fatalf("wide=%v op=%d d=%x max=%x got=%x want=%x", wide, op, d, maximum, bound, want)
					}
					rng := rand.New(rand.NewSource(19))
					if d != 0 {
						for i := 0; i < 100; i++ {
							x := rng.Uint64() & limit
							if maximum != ^uint64(0) {
								x %= maximum + 1
							}
							v := x / d
							if op == opRemU {
								v = x % d
							}
							if v > bound {
								t.Fatalf("result=%x exceeds bound=%x", v, bound)
							}
						}
					}
				}
			}
		}
	}
}
