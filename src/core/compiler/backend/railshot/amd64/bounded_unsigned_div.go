//go:build amd64

package amd64

import "os"

// Native correctness and repeated application timing qualify this selection.
// The zero override retains the ordinary constant-division path for comparison.
var boundedUnsignedDivEnabled = os.Getenv("WAGO_AMD64_BOUNDED_UNSIGNED_DIV") != "0"
var boundedRemainderLeaEnabled = os.Getenv("WAGO_AMD64_BOUNDED_REMAINDER_LEA") == "1"

// Reuse the bounded integer proof for all supported expression producers and
// current-region locals. Unknown or larger ranges retain ordinary lowering.
func (f *fn) boundedUnsignedDividend(e *elem) (uint32, bool) {
	if e == nil || e.valueType() != mtI32 {
		return 0, false
	}
	budget := 32
	maximum := f.unsignedAddressMaximum(e, false, &budget)
	return uint32(maximum), maximum > 0 && maximum <= 65535
}

// Let m=ceil(2^s/d), error=m*d-2^s. Then m*n/2^s >= n/d.
// If max*error < 2^s, its extra fractional error is less than 1/d,
// so it cannot reach the next integer for any n<=max. All proof arithmetic
// fits uint64; the generated product fits 48 bits and the immediate fits int32.
func boundedUnsignedReciprocal(max, d uint32) (uint32, byte, bool) {
	if max == 0 || max > 65535 || d < 3 || d > 0x7fffffff || d&(d-1) == 0 {
		return 0, 0, false
	}
	for s := byte(0); s <= 31; s++ {
		power := uint64(1) << s
		m := (power + uint64(d) - 1) / uint64(d)
		error := m*uint64(d) - power
		if m <= 0x7fffffff && uint64(max)*error < power {
			return uint32(m), s, true
		}
	}
	return 0, 0, false
}

func (f *fn) divConstUnsignedBounded(res Reg, d, max, multiplier uint32, shift byte, rem bool) {
	q := res
	if rem {
		q = f.allocReg(maskOf(res))
		f.pinned = f.pinned.add(q)
	}
	wide := uint64(max)*uint64(multiplier) > 0xffffffff
	if wide {
		// The dividend is a Wasm i32. Explicitly clear any host upper bits.
		f.a.MovRegReg32(res, res)
	}
	f.a.ImulRRI(q, res, int32(multiplier), wide)
	if shift != 0 {
		f.a.ShiftImm(5, q, shift, wide)
	}
	if rem {
		// q*d <= n <= max, so the low 32-bit product is exact. q is dead.
		if f.opt(optBoundedRemainderLea) && (d == 3 || d == 5 || d == 9) {
			f.a.LeaScaledW(q, q, q, uint8(log2u(uint64(d-1))), 0, false)
			f.stats.peep("bounded-remainder-lea")
		} else {
			f.a.ImulRI(q, int32(d), false)
		}
		f.a.Sub32(res, q)
		f.pinned = f.pinned.remove(q)
	}
	f.stats.peep("bounded-unsigned-div")
}
