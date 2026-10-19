//go:build amd64

package amd64

import "testing"

var addressMaximumCacheSink uint64

func TestAddressMaximumCacheIgnoresInactiveSlots(t *testing.T) {
	f := fn{policy: currentCodegenPolicy(), s: newStack()}
	e := f.pushValue(storage{kind: stLocalRef, typ: mtI32, idx: 17})
	f.localAddressMaxima[0] = localAddressMaximum{index: 17, maximum: 3}
	for _, valid := range []uint8{0, 1, 2, 128, 255} {
		f.localAddressMaximaValid = valid
		budget := 32
		want := uint64(^uint32(0))
		if valid&1 != 0 {
			want = 3
		}
		if got := f.unsignedAddressMaximum(e, false, &budget); got != want {
			t.Fatalf("mask=%x got=%d want=%d", valid, got, want)
		}
	}
}

func TestAddressMaximumCacheEveryMaskAndSlot(t *testing.T) {
	for _, wide := range []bool{false, true} {
		typ, limit := mtI32, uint64(^uint32(0))
		if wide {
			typ, limit = mtI64, ^uint64(0)
		}
		for slot := 0; slot < 8; slot++ {
			f := fn{policy: currentCodegenPolicy(), s: newStack()}
			e := f.pushValue(storage{kind: stLocalRef, typ: typ, idx: 17})
			f.localAddressMaxima[slot] = localAddressMaximum{index: 17, maximum: uint64(100 + slot)}
			for mask := 0; mask < 256; mask++ {
				f.localAddressMaximaValid = uint8(mask)
				want := limit
				if mask&(1<<slot) != 0 {
					want = uint64(100 + slot)
				}
				budget := 32
				if got := f.unsignedAddressMaximum(e, wide, &budget); got != want {
					t.Fatalf("wide=%v slot=%d mask=%x got=%d want=%d", wide, slot, mask, got, want)
				}
			}
		}
	}
}

func BenchmarkAddressMaximumCache(b *testing.B) {
	for _, tc := range []struct {
		name string
		mask uint8
	}{{"empty", 0}, {"one-miss", 1}, {"full-miss", 255}} {
		b.Run(tc.name, func(b *testing.B) {
			f := fn{policy: currentCodegenPolicy(), s: newStack()}
			e := f.pushValue(storage{kind: stLocalRef, typ: mtI32, idx: 17})
			f.localAddressMaximaValid = tc.mask
			for i := range f.localAddressMaxima {
				f.localAddressMaxima[i] = localAddressMaximum{index: uint32(i), maximum: uint64(i * 4)}
			}
			var maximum uint64
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				budget := 32
				maximum = f.unsignedAddressMaximum(e, false, &budget)
			}
			addressMaximumCacheSink = maximum
		})
	}
}
