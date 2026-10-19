//go:build amd64

package amd64

import "testing"

func TestInlineTargetPackedStandaloneFlag(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		target := inlineTarget{params: 5}
		for i, flag := range []uint32{inlineI32AddConstFlag, inlineRecursiveFlag, inlineOmitStandaloneFlag} {
			if mask&(1<<i) != 0 {
				target.params |= flag
			}
		}
		if target.paramCount() != 5 || target.isI32AddConst() != (mask&1 != 0) || target.recursive() != (mask&2 != 0) || target.omitStandalone() != (mask&4 != 0) {
			t.Fatalf("overlapping inline metadata mask=%d params=%x", mask, target.params)
		}
	}
}
