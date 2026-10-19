//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestBooleanChainOperandFolding(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		for _, or := range []bool{false, true} {
			typ, con, compare, combine := wasm.I32, byte(0x41), byte(0x48), byte(0x71)
			if wide {
				typ, con, compare = wasm.I64, 0x42, 0x53
			}
			if or {
				combine = 0x72
			}
			body := []byte{0}
			for i := byte(0); i < 4; i++ {
				body = append(body, 0x20, i, con, 7, compare)
				if i != 0 {
					body = append(body, combine)
				}
			}
			body = append(body, 0x0b)
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(mod1(t, []wasm.ValType{typ, typ, typ, typ}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"boolean-chain": true, "boolean-chain-operands": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				for mask := 0; mask < 16; mask++ {
					args := []uint64{1, 1, 1, 1}
					for i := 0; i < 4; i++ {
						if mask&(1<<i) != 0 {
							args[i] = 9
						}
					}
					want := uint64(0)
					if (!or && mask == 0) || (or && mask != 15) {
						want = 1
					}
					if got := runCompiledAmd64u(t, cm, args...); got != want {
						t.Fatalf("wide=%v or=%v enabled=%v mask=%d got=%d want=%d", wide, or, enabled, mask, got, want)
					}
				}
				if (stats.Funcs[0].Peephole["boolean-chain-operands"] > 0) != enabled {
					t.Fatalf("admission enabled=%v stats=%v", enabled, stats.Funcs[0].Peephole)
				}
				if (stats.Funcs[0].Peephole["boolean-chain-two-scratch"] > 0) != enabled {
					t.Fatal("scratch budget not selected")
				}
				cm.CodeImage.Close()
			}
		}
	}
}

func TestBooleanChainWideImmediateScratchFallback(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	body := []byte{0}
	for i := byte(0); i < 4; i++ {
		// i64.const 1<<40 cannot be encoded as a sign-extended CMP immediate.
		body = append(body, 0x20, i, 0x42, 0x80, 0x80, 0x80, 0x80, 0x80, 0x20, 0x53)
		if i != 0 {
			body = append(body, 0x71)
		}
	}
	body = append(body, 0x0b)
	for _, enabled := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(mod1(t, []wasm.ValType{wasm.I64, wasm.I64, wasm.I64, wasm.I64}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"boolean-chain": true, "boolean-chain-operands": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		if stats.Funcs[0].Peephole["boolean-chain"] == 0 || stats.Funcs[0].Peephole["boolean-chain-two-scratch"] != 0 {
			t.Fatal("large immediate did not use three-scratch fallback")
		}
		for mask := 0; mask < 16; mask++ {
			args := []uint64{1<<40 - 1, 1 << 63, ^uint64(0), 0}
			for i := 0; i < 4; i++ {
				if mask&(1<<i) != 0 {
					args[i] = 1 << 40
				}
			}
			want := uint64(0)
			if mask == 0 {
				want = 1
			}
			if got := runCompiledAmd64u(t, cm, args...); got != want {
				t.Fatalf("enabled=%v mask=%d got=%d want=%d", enabled, mask, got, want)
			}
		}
		cm.CodeImage.Close()
	}
}
