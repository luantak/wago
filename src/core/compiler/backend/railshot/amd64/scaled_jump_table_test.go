//go:build linux && amd64

package amd64

import (
	"bytes"
	"testing"
)

func TestScaledJumpTable(t *testing.T) {
	for _, raw := range []bool{false, true} {
		m := brTableIndexInRAX(t)
		if raw {
			m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 0, 0x20, 1, 0x6e}, []byte{0x20, 0}, 1)
		}
		for _, compact := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{CompactNative: compact, Stats: &stats, Optimizations: map[string]bool{"scaled-jump-table": enabled, "canonical-i32": !raw, "value-facts": !raw}})
				if err != nil {
					t.Fatal(err)
				}
				if (stats.Funcs[0].Peephole["scaled-jump-table"] != 0) != (enabled && !raw) {
					t.Fatal("missing scaled dispatch admission")
				}
				for _, idx := range []uint64{0, 1, 2, 3, 4, 5, 6, 0x80000000, 0xffffffff} {
					arg := idx
					if raw {
						arg |= 0xdeadbeef00000000
					}
					want := uint64(1000) + min(idx, 5)
					if got := runCompiledAmd64u(t, cm, arg, 1); got != want {
						t.Fatalf("raw=%v compact=%v enabled=%v idx=%x got=%d want=%d", raw, compact, enabled, arg, got, want)
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}

func TestScaledJumpTableRepeatedTargets(t *testing.T) {
	labels := []uint32{0, 0, 2, 3, 0, 1, 2}
	m := brTableLabelsInRAX(t, labels, 2)
	for _, compact := range []bool{false, true} {
		cm, err := CompileModuleWith(m, CompileOptions{CompactNative: compact, Optimizations: map[string]bool{"scaled-jump-table": true}})
		if err != nil {
			t.Fatal(err)
		}
		for idx := 0; idx < 10; idx++ {
			want := uint64(1002)
			if idx < len(labels) {
				want = 1000 + uint64(labels[idx])
			}
			if got := runCompiledAmd64u(t, cm, uint64(idx), 1); got != want {
				t.Fatalf("compact=%v idx=%d got=%d want=%d", compact, idx, got, want)
			}
		}
		cm.CodeImage.Close()
	}
}
