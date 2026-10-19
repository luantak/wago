//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestAvoidRegionalSinkSpill(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		ty, decl, xor, add, sub := wasm.I32, byte(0x7f), byte(0x73), byte(0x6a), byte(0x6b)
		if wide {
			ty, decl, xor, add, sub = wasm.I64, 0x7e, 0x85, 0x7c, 0x7d
		}
		body := []byte{1, 20, decl, 0x02, decl}
		for range 128 {
			body = append(body, 0x01)
		}
		body = append(body, 0x20, 0, 0x20, 1, 0x20, 2, xor, 0x20, 0, sub, 0x21, 0, 0x20, 1, 0x20, 2, add, 0x20, 0, sub, 0x22, 0, add, 0x0b, 0x0b)
		m := mod1(t, []wasm.ValType{ty, ty, ty}, []wasm.ValType{ty}, body)
		for _, enabled := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"avoid-sink-spill": true, "avoid-regional-sink-spill": enabled, "interval-region-pins": true, "interval-control": true}})
			if err != nil {
				t.Fatal(err)
			}
			if (stats.Funcs[0].Peephole["avoid-sink-spill"] > 0) != enabled {
				t.Fatalf("wide=%v enabled=%v admission=%v", wide, enabled, stats.Funcs[0].Peephole)
			}
			for _, args := range [][3]uint64{{0, 0, 0}, {7, 13, 29}, {0xffffffff, 0x80000000, 1}, {0xffffffffffffffff, 0x8000000000000000, 0x123456789abcdef0}} {
				want := 2*args[0] + args[1] + args[2] - (args[1] ^ args[2])
				if !wide {
					want = uint64(uint32(want))
				}
				if got := runCompiledAmd64u(t, cm, args[:]...); got != want {
					t.Fatalf("wide=%v args=%x got=%x want=%x", wide, args, got, want)
				}
			}
			cm.CodeImage.Close()
		}
	}
}
