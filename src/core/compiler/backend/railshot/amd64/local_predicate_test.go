//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestLocalBranchPredicate(t *testing.T) {
	requireCompilerDiagnostics(t)
	savedScalar, savedPredicate := sharedScalarEnabled, lonePredicateEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled, lonePredicateEnabled = savedScalar, savedPredicate }()
	for _, loop := range []bool{false, true} {
		body := []byte{0, 0x20, 0, 0x04, 0x7f, 0x41, 11, 0x05, 0x41, 22, 0x0b, 0x20, 0, 0x73, 0x0b}
		if loop {
			body = []byte{0, 0x03, 0x40, 0x20, 0, 0x41, 1, 0x6b, 0x21, 0, 0x01, 0x20, 0, 0x0d, 0, 0x0b, 0x20, 0, 0x0b}
		}
		m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, enabled := range []bool{false, true} {
			lonePredicateEnabled = enabled
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats})
			if err != nil {
				t.Fatal(err)
			}
			defer cm.CodeImage.Close()
			if (stats.Funcs[0].Peephole["lone-local-predicate"] != 0) != (enabled && loop) {
				t.Errorf("loop=%v enabled=%v missing local predicate selection: %v", loop, enabled, stats.Funcs[0].Peephole)
			}
			if loop && enabled && stats.Funcs[0].Peephole["lone-local-predicate-borrow"] == 0 {
				t.Error("stable pinned loop predicate was copied")
			}
			inputs := []uint64{0, 1, 7, 0x80000000, 0xfedcba9800000000, ^uint64(0)}
			if loop {
				inputs = []uint64{1, 2, 7, 31, 0xfedcba9800000007}
			}
			for _, input := range inputs {
				want := uint32(0)
				if !loop {
					want = 11 ^ uint32(input)
					if uint32(input) == 0 {
						want = 22
					}
				}
				if got := runCompiledAmd64u(t, cm, input); got != uint64(want) {
					t.Fatalf("loop=%v enabled=%v input=%x got=%x want=%x", loop, enabled, input, got, want)
				}
			}
		}
	}
}

func TestLocalBranchPredicateCallConvergence(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := lonePredicateEnabled
	defer func() { lonePredicateEnabled = saved }()
	for _, prefix := range []bool{false, true} {
		body := []byte{1, 1, 0x7f}
		if prefix {
			body = append(body, 0x41, 42)
		}
		body = append(body, 0x03, 0x40,
			0x20, 1, 0x20, 0, 0x6a, 0x21, 1, 0x10, 1,
			0x20, 0, 0x41, 1, 0x6b, 0x21, 0, 0x01, 0x20, 0, 0x0d, 0, 0x0b)
		if prefix {
			body = append(body, 0x1a)
		}
		body = append(body, 0x20, 1, 0x0b)
		m := modFuncs(t,
			funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: body},
			funcDef{body: []byte{0, 0x41, 7, 0x1a, 0x0b}})
		for _, enabled := range []bool{false, true} {
			lonePredicateEnabled = enabled
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"inline": false}})
			if err != nil {
				t.Fatal(err)
			}
			defer cm.CodeImage.Close()
			if (stats.Funcs[0].Peephole["lone-local-predicate"] != 0) != (enabled && !prefix) {
				t.Fatalf("prefix=%v enabled=%v selection=%v", prefix, enabled, stats.Funcs[0].Peephole)
			}
			for _, n := range []uint64{1, 2, 7, 31} {
				if got := runCompiledAmd64u(t, cm, n|0xabcdef1200000000); got != n*(n+1)/2 {
					t.Fatalf("prefix=%v enabled=%v n=%d got=%d", prefix, enabled, n, got)
				}
			}
		}
	}
}
