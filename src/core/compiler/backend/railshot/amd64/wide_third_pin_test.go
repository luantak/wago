//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestWideCallPinBudget(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{1, 67, 0x7f,
		0x20, 0, 0x45, 0x04, 0x40, 0x41, 1, 0x10, 0, 0x1a, 0x0b,
		0x20, 0, 0x21, 65,
		0x02, 0x40, 0x03, 0x40, 0x20, 65, 0x45, 0x0d, 1,
		0x20, 66, 0x20, 65, 0x6a, 0x21, 66,
		0x20, 67, 0x20, 66, 0x73, 0x21, 67,
		0x20, 65, 0x41, 1, 0x6b, 0x21, 65,
		0x0c, 0, 0x0b, 0x0b, 0x20, 67, 0x0b}
	m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, enabled := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"wide-local-pins": true, "wide-call-pin-budget": enabled, "interval-region-pins": false}})
		if err != nil {
			t.Fatal(err)
		}
		wantPins := 2
		if enabled {
			wantPins = 4
		}
		if stats.Funcs[0].PinnedLocals != wantPins {
			t.Fatalf("enabled=%v pins=%d", enabled, stats.Funcs[0].PinnedLocals)
		}
		for _, n := range []uint32{0, 1, 17, 1000} {
			var sum, want uint32
			for i := n; i > 0; i-- {
				sum += i
				want ^= sum
			}
			if got := runCompiledAmd64u(t, cm, uint64(n)); got != uint64(want) {
				t.Fatalf("enabled=%v n=%d got=%d want=%d", enabled, n, got, want)
			}
		}
		cm.CodeImage.Close()
	}
}
