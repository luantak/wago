//go:build amd64

package amd64

import "os"

var unsignedRemainderRangeEnabled = os.Getenv("WAGO_AMD64_UNSIGNED_REMAINDER_RANGE") == "1"

// Bound only constant nonzero unsigned divisors. This is a value proof, not
// a rewrite: lowering still evaluates both operands and preserves all traps
// and side effects in the dividend. Unknown producers consume the same fixed
// expression budget and conservatively return the full unsigned domain.
func (f *fn) unsignedRemainderMaximum(e *elem, wide bool, budget *int, limit uint64) uint64 {
	d := e.arg1
	if d == nil || !d.isValue() || d.st.kind != stConst || d.st.typ != e.st.typ {
		return limit
	}
	divisor := uint64(d.st.cval)
	if !wide {
		divisor = uint64(uint32(divisor))
	}
	if divisor == 0 {
		return limit
	}
	maximum := f.unsignedAddressMaximum(e.arg0, wide, budget)
	return min(maximum, divisor-1)
}
