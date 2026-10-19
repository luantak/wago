//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestStructuredLoopRangeMemory(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, variant := range []struct {
		step uint32
		cmp  byte
	}{{1, 0x47}, {2, 0x47}, {3, 0x49}, {3, 0x48}} {
		for _, base := range []uint32{65500, 65528} {
			for _, overwrite := range []bool{false, true} {
				// The declared counter starts at zero. An address carried through an
				// if result exercises the existing pure-prefix rematerialization path.
				body := []byte{1, 1, 0x7f, 0x03, 0x40, 0x20, 1, 0x41, byte(base&127) | 0x80, byte((base>>7)&127) | 0x80, byte(base >> 14), 0x6a,
					0x20, 1, 0x41, 1, 0x71, 0x04, 0x7f, 0x20, 1, 0x41, 3, 0x6a, 0x05, 0x20, 1, 0x41, 7, 0x6a, 0x0b, 0x3a, 0, 0,
					0x20, 1, 0x41, byte(variant.step), 0x6a, 0x22, 1, 0x41, 16, variant.cmp, 0x0d, 0, 0x0b}
				if overwrite {
					body = append(body, 0x20, 0, 0x21, 1, 0x20, 1, 0x2d, 0, 0)
				} else {
					body = append(body, 0x20, 1)
				}
				body = append(body, 0x0b)
				m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
				for _, on := range []bool{false, true} {
					var stats ModuleStats
					got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"structured-loop-range": on}}, func(mem []byte) { mem[0] = 0x5a }, 65536)
					want := make([]byte, 65536)
					want[0] = 0x5a
					for i := uint32(0); i < 16 && base+i < 65536; i += variant.step {
						v := i + 7
						if i&1 != 0 {
							v = i + 3
						}
						want[base+i] = byte(v)
					}
					trap := base == 65528 || overwrite
					final := uint64((16 + variant.step - 1) / variant.step * variant.step)
					if (err != nil) != trap || !trap && got != final || !bytes.Equal(mem, want) {
						t.Fatalf("base=%d overwrite=%v on=%v got=%d err=%v", base, overwrite, on, got, err)
					}
					if on && stats.Funcs[0].Peephole["structured-loop-range"] == 0 {
						t.Fatal("missed induction proof")
					}
					if (stats.Funcs[0].Peephole["memory-min-bounds"] != 0) != (on && base == 65500) {
						t.Fatalf("bounds admission base=%d on=%v stats=%v", base, on, stats.Funcs[0].Peephole)
					}
				}
			}
		}
	}
}
