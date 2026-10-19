//go:build amd64

package amd64

import "testing"

func TestRounddownReciprocal(t *testing.T) {
	state := uint32(0x12345678)
	for d := uint64(3); d < 4096; d++ {
		m, s, ok := rounddownReciprocal(d)
		if !ok {
			continue
		}
		check := func(n uint64) {
			t.Helper()
			got := (uint64(m) * (n + 1)) >> s
			if got != n/d {
				t.Fatalf("n=%d d=%d m=%d shift=%d got=%d want=%d", n, d, m, s, got, n/d)
			}
		}
		for _, n := range []uint64{0, 1, d - 1, d, d + 1, 0x7fffffff, 0xfffffffe, 0xffffffff} {
			check(n)
		}
		for i := 0; i < 128; i++ {
			state = state*1664525 + 1013904223
			check(uint64(state))
		}
	}
	for _, d := range []uint64{0, 1, 2, 4, 8, 1 << 32} {
		if _, _, ok := rounddownReciprocal(d); ok {
			t.Fatalf("unexpected admission %d", d)
		}
	}
}
