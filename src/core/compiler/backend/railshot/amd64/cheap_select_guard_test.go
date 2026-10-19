//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestCheapSelectGuard(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		typ, c, sub := wasm.I32, byte(0x41), byte(0x6b)
		if wide {
			typ, c, sub = wasm.I64, 0x42, 0x7d
		}
		for _, reverse := range []bool{false, true} {
			body := []byte{0}
			if reverse {
				body = append(body, 0x20, 0)
			}
			body = append(body, c, 0, 0x20, 0, sub)
			if !reverse {
				body = append(body, 0x20, 0)
			}
			body = append(body, 0x20, 1, 0x1b, 0x0b)
			m := mod1(t, []wasm.ValType{typ, wasm.I32}, []wasm.ValType{typ}, body)
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-pure-guard": true, "cheap-select-guard": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				if got := stats.Funcs[0].Peephole["select-pure-guard"]; (got != 0) != enabled {
					t.Fatalf("enabled=%v got=%d", enabled, got)
				}
				if shortSelectGuardEnabled && enabled && stats.Funcs[0].Peephole["short-select-guard"] != 1 {
					t.Fatal("short guard was not emitted")
				}
				for _, x := range []uint64{0, 1, 0x80000000, 0x8000000000000000, ^uint64(0)} {
					for _, cond := range []uint64{0, 1, 17} {
						want := x
						if (cond != 0) != reverse {
							want = 0 - x
						}
						if !wide {
							want = uint64(uint32(want))
						}
						if got := runCompiledAmd64u(t, cm, x, cond); got != want {
							t.Fatalf("wide=%v reverse=%v x=%x cond=%d got=%x want=%x", wide, reverse, x, cond, got, want)
						}
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}

func TestCheapSelectGuardKeepsAdditionBranchless(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, []byte{0, 0x20, 0, 0x41, 1, 0x6a, 0x20, 0, 0x20, 1, 0x1b, 0x0b})
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-pure-guard": true, "cheap-select-guard": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Peephole["select-pure-guard"] != 0 {
		t.Fatal("cheap addition introduced a branch")
	}
	for _, x := range []uint64{0, 19, 0xffffffff} {
		for _, cond := range []uint64{0, 1, 17} {
			want := uint32(x)
			if cond != 0 {
				want++
			}
			if got := runCompiledAmd64u(t, cm, x, cond); got != uint64(want) {
				t.Fatalf("x=%x cond=%d got=%x want=%x", x, cond, got, want)
			}
		}
	}
}
