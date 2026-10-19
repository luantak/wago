//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSelectFrame(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		ty, add := wasm.I32, byte(0x6a)
		if wide {
			ty, add = wasm.I64, 0x7c
		}
		params := make([]wasm.ValType, 10)
		for i := range params {
			params[i] = ty
		}
		params[9] = wasm.I32
		body := []byte{0, 0x20, 7, 0x20, 8, 0x20, 9, 0x1b}
		for i := byte(0); i < 8; i++ {
			body = append(body, 0x20, i, add, 0x20, i, add)
		}
		body = append(body, 0x0b)
		m := mod1(t, params, []wasm.ValType{ty}, body)
		for _, enabled := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-frame": enabled}})
			if err != nil {
				t.Fatal(err)
			}
			if (stats.Funcs[0].Peephole["select-frame"] > 0) != enabled {
				t.Fatalf("enabled=%v stats=%v", enabled, stats.Funcs[0].Peephole)
			}
			for _, condition := range []uint64{0, 1, 0x80000000, 0xffffffff} {
				for _, alternative := range []uint64{0, 1, 0xffffffff, 0xdeadbeef01234567} {
					args := []uint64{1, 2, 3, 4, 5, 6, 7, 8, alternative, condition}
					want := alternative + 72
					if condition != 0 {
						want = 80
					}
					if !wide {
						want = uint64(uint32(want))
					}
					if got := runCompiledAmd64u(t, cm, args...); got != want {
						t.Fatalf("wide=%v condition=%x alternative=%x got=%x want=%x", wide, condition, alternative, got, want)
					}
				}
			}
			cm.CodeImage.Close()
		}
	}
}
