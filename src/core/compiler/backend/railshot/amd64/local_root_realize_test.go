//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestLocalRootRealizePressure(t *testing.T) {
	testPressureCompactionRule(t, "local-root-realize")
}

func TestLocalRootRealize(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		typ, add, mul, c := wasm.I32, byte(0x6a), byte(0x6c), byte(0x41)
		if wide {
			typ, add, mul, c = wasm.I64, 0x7c, 0x7e, 0x42
		}
		body := []byte{0, 0x20, 0, 0x20, 1, add, 0x20, 0, mul, c, 9, 0x21, 0, 0x0b}
		m := mod1(t, []wasm.ValType{typ, typ}, []wasm.ValType{typ}, body)
		for _, enabled := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"local-root-realize": enabled}})
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range []uint64{0, 1, 0x80000000, 0xffffffff, 0x8000000000000000, 0xffffffffffffffff} {
				want := (a + 17) * a
				if !wide {
					want = uint64(uint32(want))
				}
				if got := runCompiledAmd64u(t, cm, a, 17); got != want {
					t.Fatalf("wide=%v enabled=%v a=%x got=%x want=%x", wide, enabled, a, got, want)
				}
			}
			if stats.Funcs[0].Peephole["local-root-realize"] != 0 {
				t.Fatalf("admission wide=%v enabled=%v: %v", wide, enabled, stats.Funcs[0].Peephole)
			}
			cm.CodeImage.Close()
		}
	}
}
