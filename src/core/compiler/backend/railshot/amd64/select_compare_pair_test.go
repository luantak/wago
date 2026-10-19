//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSelectComparePair(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		typ, le, ge := wasm.I32, byte(0x4c), byte(0x4e)
		if wide {
			typ, le, ge = wasm.I64, 0x57, 0x59
		}
		body := []byte{0, 0x20, 0, 0x20, 1, le, 0x20, 0, 0x20, 1, ge, 0x20, 2, 0x1b, 0x0b}
		m := mod1(t, []wasm.ValType{typ, typ, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, on := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-compare-pair": on, "local-spill-reuse": true}})
			if err != nil {
				t.Fatal(err)
			}
			if (stats.Funcs[0].Peephole["select-compare-pair"] > 0) != on {
				t.Fatalf("on=%v stats=%v", on, stats.Funcs[0].Peephole)
			}
			for _, a := range []uint64{0, 1, 0x7fffffff, 0x80000000, 0xffffffff, 1 << 63, ^uint64(0)} {
				for _, b := range []uint64{0, 7, 0x80000000, ^uint64(0)} {
					for _, condition := range []uint64{0, 1, 0x80000000, 0x100000000} {
						lt, gt := int32(a) <= int32(b), int32(a) >= int32(b)
						if wide {
							lt, gt = int64(a) <= int64(b), int64(a) >= int64(b)
						}
						want := uint64(0)
						if uint32(condition) != 0 && lt || uint32(condition) == 0 && gt {
							want = 1
						}
						if got := runCompiledAmd64u(t, cm, a, b, condition); got != want {
							t.Fatalf("on=%v wide=%v a=%x b=%x c=%x got=%d want=%d", on, wide, a, b, condition, got, want)
						}
					}
				}
			}
			cm.CodeImage.Close()
		}
	}
}
