//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/src/core/runtime"
	"testing"
	"unsafe"
)

func recursiveBudgetModule(t *testing.T) *wasm.Module {
	return mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I64}, []byte{0, 0x20, 0, 0x45, 0x04, 0x7e, 0x42, 3, 0x05, 0x20, 0, 0xad, 0x20, 0, 0x41, 1, 0x6b, 0x10, 0, 0x7c, 0x0b, 0x0b})
}

func TestRecursiveInlineFourthBankBound(t *testing.T) {
	saved := recursiveInlineBudgetEnabled
	recursiveInlineBudgetEnabled = true
	defer func() { recursiveInlineBudgetEnabled = saved }()
	f := fn{policy: currentCodegenPolicy(), inlineDepth: 3, inlineRecursiveBytes: 480}
	target := inlineTarget{params: inlineRecursiveFlag, body: make([]byte, 32)}
	if !f.allowInlineDepth(&target) {
		t.Fatal("fourth bank declined with source expansion exactly within the unchanged 512-byte budget")
	}
	f.inlineRecursiveBytes++
	if f.allowInlineDepth(&target) {
		t.Fatal("source expansion exceeded 512 bytes")
	}
	f.inlineRecursiveBytes = 0
	f.inlineDepth = 4
	if f.allowInlineDepth(&target) {
		t.Fatal("fifth bank admitted")
	}
	f.inlineDepth = 3
	target.params, target.carriesCalls = 0, true
	if f.allowInlineDepth(&target) {
		t.Fatal("nonrecursive wrapper admitted at recursive depth")
	}
}

func TestRecursiveInlineShareEnd(t *testing.T) {
	if got := unsafe.Offsetof(fn{}.inlineRetFrame) - unsafe.Offsetof(fn{}.inlineRecursiveBytes); got != 8 {
		t.Fatalf("recursive state word grew: %d", got)
	}
	for _, tc := range []struct {
		used  uint16
		roots uint8
		want  uint16
	}{{0, 2, 256}, {232, 1, 512}, {512, 2, 512}, {0, 1, 512}, {0, 0, 512}, {16, 2, 264}} {
		if got := recursiveInlineShareEnd(tc.used, tc.roots); got != tc.want {
			t.Fatalf("used=%d roots=%d got=%d want=%d", tc.used, tc.roots, got, tc.want)
		}
	}
	saved := recursiveInlineBudgetEnabled
	recursiveInlineBudgetEnabled = true
	defer func() { recursiveInlineBudgetEnabled = saved }()
	f := fn{policy: currentCodegenPolicy(), inlineDepth: 1, inlineRecursiveBytes: 232, inlineRecursiveLimit: 256}
	target := inlineTarget{params: inlineRecursiveFlag, body: make([]byte, 32)}
	if f.allowInlineDepth(&target) {
		t.Fatal("first root exceeded its fair share")
	}
	f.inlineRecursiveLimit = 512
	if !f.allowInlineDepth(&target) {
		t.Fatal("later root could not use remaining bytes")
	}
}

func TestRecursiveInlineShareWorkers(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{0, 0x20, 0, 0x41, 2, 0x48, 0x04, 0x7e, 0x20, 0, 0xac, 0x05,
		0x20, 0, 0x41, 1, 0x6b, 0x10, 1, 0x20, 0, 0x41, 2, 0x6b, 0x10, 1, 0x7c, 0x0b, 0x0b}
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I64}, body: []byte{0, 0x20, 0, 0x10, 1, 0x0b}},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I64}, body: body})
	for _, workers := range []int{1, 2} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Workers: workers, Stats: &stats, Optimizations: map[string]bool{"recursive-inline-budget": true}})
		if err != nil {
			t.Fatal(err)
		}
		if cm.CodeImage != nil {
			defer cm.CodeImage.Close()
		}
		if stats.Funcs[0].Peephole["recursive-inline-root-budget"] != 0 || stats.Funcs[1].Peephole["recursive-inline-root-budget"] != 2 {
			t.Fatalf("workers=%d budget state leaked across functions", workers)
		}
		a, b := uint64(0), uint64(1)
		for n := uint64(0); n <= 17; n++ {
			if got := runCompiledAmd64u(t, cm, n); got != a {
				t.Fatalf("workers=%d n=%d got=%d want=%d", workers, n, got, a)
			}
			a, b = b, a+b
		}
	}
}

func TestRecursiveInlineBudgetWithUnusedImport(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := recursiveBudgetModule(t)
	m.Exports = nil
	m.Imports = []wasm.Import{{Type: wasm.NewFuncExternType(wasm.TypeIdx{Index: 0})}}
	m.Code[0].BodyBytes = bytes.ReplaceAll(m.Code[0].BodyBytes, []byte{0x10, 0}, []byte{0x10, 1})
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"recursive-inline-budget": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Peephole["recursive-inline-nested"] == 0 {
		t.Fatal("unrelated import blocked pure recursion")
	}
	if got := runCompiledAmd64u(t, cm, 17); got != 156 {
		t.Fatalf("got=%d", got)
	}
}

func TestRecursiveInlineBudgetRejectsNonIntegerBody(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := recursiveBudgetModule(t)
	m.Code[0].Locals.Runs = []wasm.LocalRun{{Count: 1, Type: wasm.F64}}
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"recursive-inline-budget": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Peephole["recursive-inline-nested"] != 0 {
		t.Fatal("non-integer local admitted")
	}
	if got := runCompiledAmd64u(t, cm, 17); got != 156 {
		t.Fatalf("got=%d", got)
	}
}

func TestRecursiveInlineBudget(t *testing.T) {
	requireCompilerDiagnostics(t)
	var stats ModuleStats
	cm, err := CompileModuleWith(recursiveBudgetModule(t), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"recursive-inline-budget": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Peephole["recursive-inline-nested"] == 0 {
		t.Fatal("bounded nested recursion was not inlined")
	}
	for _, v := range []uint64{0, 1, 2, 17, 100} {
		if got := runCompiledAmd64u(t, cm, v); got != 3+v*(v+1)/2 {
			t.Fatalf("n=%d got=%d", v, got)
		}
	}
}

func TestRecursiveInlineBudgetLocalBanks(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{1, 1, 0x7f, 0x20, 0, 0x21, 1, 0x20, 0, 0x41, 2, 0x48, 0x04, 0x7e,
		0x20, 0, 0xac, 0x05, 0x20, 0, 0x41, 1, 0x6b, 0x10, 1,
		0x20, 1, 0x41, 2, 0x6b, 0x10, 1, 0x7c, 0x0b, 0x0b}
	// Reusing a bank must clear its declared local before every new splice.
	body = append(append(append([]byte(nil), body[:3]...), 0x20, 1, 0x04, 0x7e, 0x42, 0x7f, 0x05), body[3:]...)
	body = append(body, 0x0b)
	// A second function and a declared local exercise the generalized eligibility
	// and independent local banks, not the legacy single-function class.
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I64}, body: []byte{0, 0x20, 0, 0x10, 1, 0x0b}},
		funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I64}, body: body})
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"recursive-inline-budget": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[1].Peephole["recursive-inline-nested"] == 0 || stats.Funcs[1].Calls["regabi"] == 0 {
		t.Fatal("nested expansion or residual calls missing")
	}
	a, b := uint64(0), uint64(1)
	for n := uint64(0); n <= 17; n++ {
		if got := runCompiledAmd64u(t, cm, n); got != a {
			t.Fatalf("n=%d got=%d want=%d", n, got, a)
		}
		a, b = b, a+b
	}
}

func TestRecursiveInlineBudgetParallelParameters(t *testing.T) {
	for _, n := range []uint64{0, 1, 2, 7} {
		for _, a := range []uint64{3, 0x80000000, 0xffffffff} {
			for _, b := range []uint64{17, 0x12345678} {
				body := []byte{0, 0x20, 0, 0x45, 0x04, 0x7e, 0x20, 1, 0xad, 0x42, 32, 0x86, 0x20, 2, 0xad, 0x84, 0x05,
					0x20, 0, 0x41, 1, 0x6b, 0x20, 2, 0x20, 1, 0x10, 0,
					0x20, 1, 0xad, 0x20, 2, 0xad, 0x7c, 0x7c, 0x0b, 0x0b}
				m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I64}, body)
				cm, err := CompileModuleWith(m, CompileOptions{Optimizations: map[string]bool{"recursive-inline-budget": true}})
				if err != nil {
					t.Fatal(err)
				}
				got := runCompiledAmd64u(t, cm, n, a|0xfeedbeef00000000, b|0xdeadbeef00000000)
				cm.CodeImage.Close()
				x, y := a, b
				if n&1 != 0 {
					x, y = y, x
				}
				want := (x<<32 | y) + n*(a+b)
				if got != want {
					t.Fatalf("n=%d a=%x b=%x got=%x want=%x", n, a, b, got, want)
				}
			}
		}
	}
}

func TestRecursiveInlineBudgetStopsExpansion(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{0}
	for i := 0; i < 90; i++ {
		body = append(body, 1)
	}
	body = append(body, 0x20, 0, 0x41, 2, 0x48, 0x04, 0x7e, 0x20, 0, 0xac, 0x05,
		0x20, 0, 0x41, 1, 0x6b, 0x10, 0, 0x20, 0, 0x41, 2, 0x6b, 0x10, 0, 0x7c, 0x0b, 0x0b)
	m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I64}, body)
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"recursive-inline-budget": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	n := stats.Funcs[0].Peephole["recursive-inline-nested"]
	size := len(m.Code[0].BodyBytes)
	if n == 0 || n*size > maxRecursiveInlineBytes || (n+1)*size <= maxRecursiveInlineBytes || stats.Funcs[0].Calls["regabi"] == 0 {
		t.Fatalf("nested=%d body=%d calls=%v", n, size, stats.Funcs[0].Calls)
	}
	if got := runCompiledAmd64u(t, cm, 10); got != 55 {
		t.Fatalf("got=%d", got)
	}
}

func TestRecursiveInlineBudgetKeepsStackTrap(t *testing.T) {
	m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, []byte{0, 0x20, 0, 0x10, 0, 0x41, 1, 0x6a, 0x0b})
	for _, enabled := range []bool{false, true} {
		cm, err := CompileModuleWith(m, CompileOptions{Optimizations: map[string]bool{"recursive-inline-budget": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		eng, err := runtime.NewEngine()
		if err != nil {
			t.Fatal(err)
		}
		defer eng.Close()
		jm, err := runtime.NewJobMemory(65536)
		if err != nil {
			t.Fatal(err)
		}
		defer jm.Close()
		// Raw low-level test entry does not install the fence automatically.
		jm.SetStackFence(eng.StackLimit())
		arena, err := runtime.NewArena(4096)
		if err != nil {
			t.Fatal(err)
		}
		defer arena.Close()
		code, entry, err := runtime.MapCode(cm.Code)
		if err != nil {
			t.Fatal(err)
		}
		defer runtime.Unmap(code)
		args, result, trap := arena.Alloc(128), arena.Alloc(128), arena.Alloc(runtime.TrapBufferBytes)
		binary.LittleEndian.PutUint64(args, 1)
		err = eng.Call(entry+uintptr(cm.Entry[0]), args, jm.LinearMemory(), trap, result)
		te, ok := err.(*runtime.TrapError)
		if !ok || te.Code != runtime.TrapStackFenceBreached {
			t.Fatalf("unbounded ordinary recursion: %v", err)
		}
	}
}
