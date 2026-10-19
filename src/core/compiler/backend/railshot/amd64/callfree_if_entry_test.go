//go:build linux && amd64

package amd64

import (
	"math"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestCallFreeIfEntrySnapshot(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, prefix := range []bool{false, true} {
		for _, callInside := range []bool{false, true} {
			body := []byte{1, 7, 0x7f, 0x20, 0, 0x10, 1, 0x21, 0,
				0x02, 0x40, 0x03, 0x40}
			if prefix {
				body = append(body, 0x20, 0)
			}
			body = append(body, 0x20, 1, 0x20, 0, 0x6a, 0x21, 1,
				0x20, 0, 0x04, 0x40)
			if callInside {
				body = append(body, 0x20, 1, 0x10, 1, 0x21, 1)
			}
			body = append(body, 0x20, 0, 0x41, 1, 0x6b, 0x22, 0, 0x0d, 1, 0x0c, 2, 0x0b)
			if prefix {
				body = append(body, 0x1a)
			}
			body = append(body, 0x0b, 0x0b, 0x20, 1, 0x10, 1, 0x0b)
			m := modFuncs(t,
				funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: body},
				funcDef{params: []wasm.ValType{wasm.I32}, results: []wasm.ValType{wasm.I32}, body: []byte{0, 0x20, 0, 0x0b}})
			for _, on := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats,
					Optimizations: map[string]bool{"inline": false, "callfree-loop-reg-state": on}})
				if err != nil {
					t.Fatal(err)
				}
				if (stats.Funcs[0].Peephole["callfree-if-entry"] > 0) != (on && !callInside) {
					t.Fatalf("prefix=%v callInside=%v on=%v stats=%v", prefix, callInside, on, stats.Funcs[0].Peephole)
				}
				for _, n := range []uint64{0, 1, 2, 7, 31, 65, 0xfeedbeef00000007} {
					value := uint64(uint32(n))
					if got, want := runCompiledAmd64u(t, cm, n), value*(value+1)/2; got != want {
						t.Fatalf("prefix=%v callInside=%v on=%v n=%x got=%d want=%d", prefix, callInside, on, n, got, want)
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}

func TestCallFreeIfEntryFloatPins(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := modFuncs(t,
		funcDef{params: []wasm.ValType{wasm.I32, wasm.F64}, results: []wasm.ValType{wasm.F64}, body: []byte{
			2, 7, 0x7f, 1, 0x7c,
			0x20, 1, 0x10, 1, 0x21, 1,
			0x02, 0x40, 0x03, 0x40, 0x20, 0, 0x04, 0x40,
			0x20, 9, 0x20, 1, 0xa0, 0x21, 9,
			0x20, 0, 0x41, 1, 0x6b, 0x22, 0, 0x0d, 1, 0x0c, 2,
			0x0b, 0x0b, 0x0b, 0x20, 9, 0x10, 1, 0x0b}},
		funcDef{params: []wasm.ValType{wasm.F64}, results: []wasm.ValType{wasm.F64}, body: []byte{0, 0x20, 0, 0x0b}})
	for _, on := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{
			"inline": false, "callfree-loop-reg-state": on}})
		if err != nil {
			t.Fatal(err)
		}
		if (stats.Funcs[0].Peephole["callfree-if-entry"] > 0) != on {
			t.Fatalf("on=%v stats=%v", on, stats.Funcs[0].Peephole)
		}
		for _, n := range []uint64{0, 1, 2, 7, 31, 0xfeedbeef00000007} {
			for _, x := range []float64{0.25, -1.5, 1 << 20} {
				want := float64(0)
				for i := uint32(0); i < uint32(n); i++ {
					want += x
				}
				if got := runCompiledAmd64u(t, cm, n, math.Float64bits(x)); got != math.Float64bits(want) {
					t.Fatalf("on=%v n=%x x=%g got=%x want=%x", on, n, x, got, math.Float64bits(want))
				}
			}
		}
		cm.CodeImage.Close()
	}
}

func TestCallFreeIfEntryElseMerge(t *testing.T) {
	requireCompilerDiagnostics(t)
	params := []wasm.ValType{wasm.I32}
	m := modFuncs(t,
		funcDef{params: params, results: params, body: []byte{
			1, 7, 0x7f, 0x20, 0, 0x10, 1, 0x21, 0,
			0x02, 0x40, 0x03, 0x40,
			0x20, 0, 0x45, 0x0d, 1,
			0x20, 1, 0x41, 1, 0x6a, 0x21, 1,
			0x20, 0, 0x41, 1, 0x71, 0x04, 0x40,
			0x20, 1, 0x41, 3, 0x6a, 0x21, 1,
			0x05, 0x20, 1, 0x41, 7, 0x6a, 0x21, 1, 0x0b,
			0x20, 0, 0x41, 1, 0x6b, 0x21, 0, 0x0c, 0,
			0x0b, 0x0b, 0x20, 1, 0x10, 1, 0x0b}},
		funcDef{params: params, results: params, body: []byte{0, 0x20, 0, 0x0b}})
	for _, on := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{
			"inline": false, "callfree-loop-reg-state": on}})
		if err != nil {
			t.Fatal(err)
		}
		if stats.Funcs[0].Peephole["callfree-if-entry"] != 0 {
			t.Fatal("snapshot without a promoted loop header", stats.Funcs[0].Peephole)
		}
		for _, n := range []uint64{0, 1, 2, 3, 7, 31, 64, 0xfeedbeef00000007} {
			value := uint64(uint32(n))
			want := value/2*7 + (value+1)/2*3 + value
			if got := runCompiledAmd64u(t, cm, n); got != want {
				t.Fatalf("on=%v n=%x got=%d want=%d", on, n, got, want)
			}
		}
		cm.CodeImage.Close()
	}
}
