//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestInlineLeafWrapperRepeatedCalls(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x10, 1, 0x20, 0, 0x10, 1, 0x6a, 0x20, 0, 0x6a, 0x0b}},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{
			0, 0x20, 0, 0x41, 0, 0x48, 0x04, 0x40,
			0x20, 0, 0x10, 2, 0x41, 1, 0x6a, 0x0f, 0x0b,
			0x20, 0, 0x10, 2, 0x0b,
		}},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x41, 7, 0x6c, 0x0b}},
	)
	for _, enabled := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"inline": true, "inline-leaf-wrapper": enabled, "inline-parameter-pins": true}})
		if err != nil {
			t.Fatal(err)
		}
		if got := stats.Funcs[0].Calls[callKindInline]; (got == 2) != enabled {
			t.Fatalf("enabled=%v inline calls=%d", enabled, got)
		}
		for _, n := range []uint32{0, 1, 19, 1 << 31, ^uint32(0)} {
			want := n * 15
			if int32(n) < 0 {
				want += 2
			}
			if got := uint32(runCompiledAmd64u(t, cm, uint64(n))); got != want {
				t.Fatalf("enabled=%v n=%x got=%x want=%x", enabled, n, got, want)
			}
		}
		cm.CodeImage.Close()
	}
}

// A result assigned to an inline parameter must not overwrite the caller's
// same-numbered local through call/local.set fusion.
func TestInlineLeafWrapperParameterWrite(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x10, 1, 0x20, 0, 0x6a, 0x0b}},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x10, 2, 0x21, 0, 0x20, 0, 0x0b}},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x41, 7, 0x6c, 0x0b}},
	)
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"inline": true, "inline-leaf-wrapper": true, "inline-parameter-pins": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Calls[callKindInline] != 1 {
		t.Fatal("wrapper was not inlined")
	}
	for _, n := range []uint32{0, 1, 19, 1 << 31, ^uint32(0)} {
		if got := uint32(runCompiledAmd64u(t, cm, uint64(n))); got != n*8 {
			t.Fatalf("n=%x got=%x", n, got)
		}
	}
}

func TestInlineLeafWrapperExpansionBudget(t *testing.T) {
	requireCompilerDiagnostics(t)
	caller := []byte{0}
	for range 8 {
		caller = append(caller, 0x20, 0, 0x10, 1, 0x1a)
	}
	caller = append(caller, 0x20, 0, 0x0b)
	wrapper := []byte{0, 0x20, 0, 0x10, 2}
	for range 8 {
		wrapper = append(wrapper, 0x41, 1, 0x6a)
	}
	wrapper = append(wrapper, 0x0b)
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: caller},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: wrapper},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x41, 7, 0x6c, 0x0b}},
	)
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"inline": true, "inline-leaf-wrapper": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Calls[callKindInline] != 0 {
		t.Fatal("duplicated wrapper exceeded aggregate budget")
	}
	if got := runCompiledAmd64u(t, cm, 19); got != 19 {
		t.Fatalf("got %d", got)
	}
}
