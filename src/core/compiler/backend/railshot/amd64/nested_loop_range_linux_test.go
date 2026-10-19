//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
	"testing"
)

func TestNestedLoopRangeMemoryAndTraps(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, base := range []int64{65500, 65528} {
		for _, overwrite := range []bool{false, true} {
			body := []byte{1, 3, 0x7f, 0x03, 0x40, 0x41, 0, 0x21, 1, 0x03, 0x40,
				0x20, 0, 0x41, 4, 0x6c, 0x20, 1, 0x6a, 0x41}
			body = append(body, wasmtest.SLEB64(base)...)
			body = append(body, 0x6a, 0x20, 0, 0x41, 4, 0x6c, 0x20, 1, 0x6a, 0x41, 1, 0x6a, 0x3a, 0, 0,
				0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x41, 4, 0x47, 0x0d, 0, 0x0b,
				0x20, 0, 0x41, 1, 0x6a, 0x22, 0, 0x41, 4, 0x47, 0x0d, 0, 0x0b)
			if overwrite {
				body = append(body, 0x41, 0x80, 0x80, 4, 0x21, 0, 0x20, 0, 0x2d, 0, 0)
			} else {
				body = append(body, 0x20, 0)
			}
			body = append(body, 0x0b)
			m := modMem(t, 1, nil, []wasm.ValType{wasm.I32}, body)
			want := make([]byte, 65536)
			for i := int64(0); i < 16 && base+i < 65536; i++ {
				want[base+i] = byte(i + 1)
			}
			for _, on := range []bool{false, true} {
				var stats ModuleStats
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"structured-loop-range": true, "nested-loop-range": on}}, nil)
				trap := base == 65528 || overwrite
				if (err != nil) != trap || !trap && got != 4 || !bytes.Equal(mem, want) {
					t.Fatalf("base=%d overwrite=%v on=%v got=%d err=%v", base, overwrite, on, got, err)
				}
				if on && stats.Funcs[0].Peephole["structured-loop-range"] < 2 {
					t.Fatal("missed simultaneous induction proofs")
				}
				if on && base == 65500 && stats.Funcs[0].Peephole["memory-min-bounds"] == 0 {
					t.Fatal("missed multidimensional minimum-memory bounds proof")
				}
			}
		}
	}
}
