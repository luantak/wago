//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestAvoidLocalSinkSpill(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		typ, decl, xor, add, sub := wasm.I32, byte(0x7f), byte(0x73), byte(0x6a), byte(0x6b)
		if wide {
			typ, decl, xor, add, sub = wasm.I64, 0x7e, 0x85, 0x7c, 0x7d
		}
		// Two different expression trees share only a destination constraint:
		// x=(a^b)-x; x=(a+b)-x. No arithmetic idiom is recognized by lowering.
		// Keep the original x live beneath both assignments as an independent
		// operand-stack value: destination placement must preserve its get-time value.
		body := []byte{1, 6, decl, 0x20, 0, 0x20, 1, 0x20, 2, xor, 0x20, 0, sub, 0x21, 0,
			0x20, 1, 0x20, 2, add, 0x20, 0, sub, 0x22, 0, add, 0x0b}
		m := mod1(t, []wasm.ValType{typ, typ, typ}, []wasm.ValType{typ}, body)
		var spills [2]int
		for i, enabled := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"avoid-sink-spill": enabled, "interval-region-pins": false}})
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][3]uint64{{0, 0, 0}, {7, 13, 29}, {0xffffffff, 0x80000000, 1}, {0xffffffffffffffff, 0x8000000000000000, 0x123456789abcdef0}} {
				want := 2*args[0] + args[1] + args[2] - (args[1] ^ args[2])
				if !wide {
					want = uint64(uint32(want))
				}
				if got := runCompiledAmd64u(t, cm, args[:]...); got != want {
					t.Fatalf("wide=%v enabled=%v args=%x got=%x want=%x", wide, enabled, args, got, want)
				}
			}
			spills[i] = stats.Funcs[0].Spills
			if (stats.Funcs[0].Peephole["avoid-sink-spill"] > 0) != enabled {
				t.Fatalf("admission wide=%v enabled=%v stats=%v", wide, enabled, stats.Funcs[0].Peephole)
			}
			cm.CodeImage.Close()
		}
		if spills[1] >= spills[0] {
			t.Fatalf("wide=%v spills off/on=%v", wide, spills)
		}
	}
}
