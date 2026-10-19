//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestInlineParameterPinsAcrossSites(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{
			0, 0x20, 0, 0x41, 7, 0x10, 1, 0x20, 0, 0x41, 13, 0x10, 1, 0x6a, 0x0b,
		}},
		funcDef{params: []wasm.ValType{wasm.I32, wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x20, 1, 0x6c, 0x0b}},
	)
	for _, enabled := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"inline": true, "inline-parameter-pins": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		if got := stats.Funcs[0].Peephole["inline-parameter-pin"]; (got != 0) != enabled {
			t.Fatalf("enabled=%v pins=%d", enabled, got)
		}
		for _, n := range []uint32{0, 1, 19, 1 << 31, ^uint32(0)} {
			if got := uint32(runCompiledAmd64u(t, cm, uint64(n))); got != n*20 {
				t.Fatalf("enabled=%v n=%x got=%x", enabled, n, got)
			}
		}
		cm.CodeImage.Close()
	}
}
