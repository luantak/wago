//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
	"testing"
)

func TestMaskedTruth(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, mask := range []uint32{0, 1, 2, 0x80, 0x80000000, 0xdeadbeef, 0xffffffff} {
		for _, mode := range []string{"if", "br-if", "select", "guard"} {
			body := []byte{0}
			if mode == "br-if" {
				body = append(body, 0x02, 0x40)
			}
			if mode == "select" {
				body = append(body, 0x41, 7, 0x41, 11)
			}
			if mode == "guard" {
				body = append(body, 0x20, 0, 0x41, 7, 0x6c, 0x41, 3, 0x73, 0x41, 11, 0x6c, 0x41, 11)
			}
			body = append(body, 0x20, 0, 0x41)
			body = append(body, wasmtest.SLEB32(int32(mask))...)
			body = append(body, 0x71)
			switch mode {
			case "if":
				body = append(body, 0x04, 0x7f, 0x41, 7, 0x05, 0x41, 11, 0x0b)
			case "br-if":
				body = append(body, 0x0d, 0, 0x41, 11, 0x0f, 0x0b, 0x41, 7)
			default:
				body = append(body, 0x1b)
			}
			body = append(body, 0x0b)
			var stats ModuleStats
			cm, err := CompileModuleWith(mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"masked-truth": true, "select-pure-guard": true}})
			if err != nil {
				t.Fatal(err)
			}
			defer cm.CodeImage.Close()
			wantFused := 1
			if mask == 0 || mask == 0xffffffff || mode == "if" || mode == "br-if" {
				wantFused = 0
			}
			if stats.Funcs[0].Peephole["masked-truth"] != wantFused {
				t.Fatalf("%s mask=%x: masked predicate not fused", mode, mask)
			}
			for _, x := range []uint32{0, 1, 2, 127, 128, 0x80000000, 0xffffffff} {
				want := uint32(11)
				if x&mask != 0 {
					want = 7
					if mode == "guard" {
						want = ((x * 7) ^ 3) * 11
					}
				}
				if got := runCompiledAmd64u(t, cm, uint64(x)|0x1234567800000000); got != uint64(want) {
					t.Fatalf("%s mask=%x x=%x got=%x want=%x", mode, mask, x, got, want)
				}
			}
		}
	}
}

func TestMaskedTruthPreservesValue(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, store := range []bool{false, true} {
		body := []byte{0}
		if store {
			body = append(body, 0x41, 0)
		}
		body = append(body, 0x20, 0, 0x41)
		body = append(body, wasmtest.SLEB32(0xb6)...)
		body = append(body, 0x71)
		if store {
			body = append(body, 0x3a, 0, 0, 0x41, 0, 0x2d, 0, 0)
		}
		body = append(body, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, x := range []uint64{0, 1, 2, 0x80, 0xffffffff} {
			got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"masked-truth": true}}, nil, x)
			if err != nil || got != x&0xb6 {
				t.Fatalf("store=%v x=%x got=%x err=%v", store, x, got, err)
			}
		}
	}
}
