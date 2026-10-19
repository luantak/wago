//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestRangeTableDispatchBranchResults(t *testing.T) {
	requireCompilerDiagnostics(t)
	// Each target consumes the same branch value, but reaches a different
	// suffix of additions. Transfers must preserve the value on every edge.
	body := []byte{0, 2, 0x7f, 2, 0x7f, 2, 0x7f, 0x20, 1, 0x20, 0,
		0x0e, 9, 0, 1, 1, 1, 1, 1, 1, 1, 2, 2,
		0x0b, 0x41, 1, 0x6a, 0x0b, 0x41, 2, 0x6a, 0x0b, 0x41, 4, 0x6a, 0x0b}
	m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, compact := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{CompactNative: compact, Stats: &stats})
		if err != nil {
			t.Fatal(err)
		}
		if (stats.Funcs[0].Peephole["range-table-dispatch"] > 0) != !compact {
			t.Fatalf("compact=%v stats=%v", compact, stats.Funcs[0].Peephole)
		}
		for _, idx := range []uint64{0, 1, 7, 8, 9, 0x80000000, 0xffffffff, 0xfeedbeef00000001} {
			add := uint32(4)
			if uint32(idx) == 0 {
				add = 7
			} else if uint32(idx) < 8 {
				add = 6
			}
			for _, value := range []uint64{0, 1, 0x7fffffff, 0xffffffff, 0xdeadbeef00000042} {
				if got, want := runCompiledAmd64u(t, cm, idx, value), uint64(uint32(value)+add); got != want {
					t.Fatalf("compact=%v idx=%x value=%x got=%x want=%x", compact, idx, value, got, want)
				}
			}
		}
		cm.CodeImage.Close()
	}
}

func TestBranchTableRangePlan(t *testing.T) {
	for length := 0; length <= 256; length++ {
		for period := 1; period <= 17; period++ {
			labels := make([]uint32, length)
			for i := range labels {
				labels[i] = uint32(i/period) % 3
			}
			ranges, n := branchTableRanges(labels)
			if n == 0 {
				continue
			}
			if length < 8 || n > 3 || length < 3*n || ranges[n-1].last != length-1 {
				t.Fatalf("bad admission length=%d period=%d ranges=%v n=%d", length, period, ranges, n)
			}
			start := 0
			for _, run := range ranges[:n] {
				for i := start; i <= run.last; i++ {
					if labels[i] != run.label {
						t.Fatalf("bad partition length=%d period=%d index=%d", length, period, i)
					}
				}
				start = run.last + 1
			}
		}
	}
}

func TestRangeTableDispatch(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, labels := range [][]uint32{
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 1, 1, 1, 1, 1, 1},
		{0, 1, 1, 1, 1, 1, 1, 1, 2},
		{0, 3, 3, 3, 3, 3, 3, 3, 0},
		{3, 3, 3, 3, 3, 3, 3, 3, 3},
		{0, 1, 0, 1, 0, 1, 0, 1, 0},
	} {
		m := brTableLabelsInRAX(t, labels, 3)
		for _, compact := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{CompactNative: compact, Stats: &stats})
			if err != nil {
				t.Fatal(err)
			}
			wantRange := !compact && labels[3] == labels[4]
			if (stats.Funcs[0].Peephole["range-table-dispatch"] > 0) != wantRange {
				t.Fatalf("labels=%v compact=%v stats=%v", labels, compact, stats.Funcs[0].Peephole)
			}
			wantSingletons := wantRange && labels[3] == 3
			if (stats.Funcs[0].Peephole["range-table-singletons"] > 0) != wantSingletons {
				t.Fatalf("default-elision labels=%v compact=%v stats=%v", labels, compact, stats.Funcs[0].Peephole)
			}
			for _, x := range []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 0x7fffffff, 0x80000000, 0xffffffff, 0xfeedbeef00000001} {
				label := uint32(3)
				if uint32(x) < uint32(len(labels)) {
					label = labels[uint32(x)]
				}
				if got := runCompiledAmd64u(t, cm, x, 1); got != uint64(1000+label) {
					t.Fatalf("labels=%v compact=%v x=%x got=%d want=%d", labels, compact, x, got, 1000+label)
				}
			}
			cm.CodeImage.Close()
		}
	}
}
