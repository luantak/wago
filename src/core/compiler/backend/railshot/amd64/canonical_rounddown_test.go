//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestCanonicalRounddown(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, op := range []byte{0x6e, 0x70} {
		for _, arithmetic := range []bool{false, true} {
			body := []byte{0, 0x20, 0}
			if arithmetic {
				body = append(body, 0x41, 1, 0x6a)
			}
			body = append(body, 0x41, 7, op, 0x0b)
			m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"canonical-rounddown": enabled, "rounddown-div": true, "canonical-i32": true}})
				if err != nil {
					t.Fatal(err)
				}
				if (stats.Funcs[0].Peephole["canonical-rounddown"] > 0) != enabled {
					t.Fatalf("enabled=%v arithmetic=%v stats=%v", enabled, arithmetic, stats.Funcs[0].Peephole)
				}
				for _, n := range []uint64{0, 1, 6, 7, 8, 0x7fffffff, 0xfffffffe, 0xffffffff, 0xdeadbeefffffffff} {
					v := uint32(n)
					if arithmetic {
						v++
					}
					want := v / 7
					if op == 0x70 {
						want = v % 7
					}
					if got := uint32(runCompiledAmd64u(t, cm, n)); got != want {
						t.Fatalf("n=%x op=%x got=%d want=%d", n, op, got, want)
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}
