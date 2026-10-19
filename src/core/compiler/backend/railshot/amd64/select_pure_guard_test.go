//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSelectPureGuard(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		ty, cn, mul, xor := wasm.I32, byte(0x41), byte(0x6c), byte(0x73)
		if wide {
			ty, cn, mul, xor = wasm.I64, 0x42, 0x7e, 0x85
		}
		for _, reverse := range []bool{false, true} {
			work := []byte{0x20, 0, cn, 7, mul, cn, 3, xor, cn, 11, mul}
			body := []byte{0}
			if reverse {
				body = append(body, 0x20, 1)
			}
			body = append(body, work...)
			if !reverse {
				body = append(body, 0x20, 1)
			}
			body = append(body, 0x20, 2, 0x1b, 0x20, 1, xor, 0x0b)
			m := mod1(t, []wasm.ValType{ty, ty, wasm.I32}, []wasm.ValType{ty}, body)
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-pure-guard": true}})
			if err != nil {
				t.Fatal(err)
			}
			defer cm.CodeImage.Close()
			if stats.Funcs[0].Peephole["select-pure-guard"] != 1 {
				t.Fatal("expensive pure select was not guarded")
			}
			for _, c := range []uint64{0, 1, 2, 0xffffffff} {
				for _, a := range []uint64{0, 1, 0xffffffff, 0x8123456789abcdef} {
					b := uint64(0xfedcba9876543210)
					want := b
					if (c != 0) != reverse {
						want = ((a * 7) ^ 3) * 11
					}
					want ^= b
					if !wide {
						want = uint64(uint32(want))
					}
					if got := runCompiledAmd64u(t, cm, a, b, c); got != want {
						t.Fatalf("wide=%v reverse=%v a=%x c=%x got=%x want=%x", wide, reverse, a, c, got, want)
					}
				}
			}
		}
	}
}

func TestSelectPureGuardPressure(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, predicate := range []byte{0x6e, 0x74} { // unsigned divide / variable shift
		body := []byte{0}
		// Keep many computed values live across the new conditional edge.
		for i := 0; i < 24; i++ {
			body = append(body, 0x20, 0, 0x41, byte(i+1), 0x6c)
		}
		body = append(body, 0x20, 0, 0x41, 7, 0x6c, 0x41, 3, 0x73, 0x41, 11, 0x6c, 0x20, 1, 0x20, 2, 0x20, 3, predicate, 0x1b)
		for i := 0; i < 24; i++ {
			body = append(body, 0x73)
		}
		body = append(body, 0x20, 1, 0x73, 0x0b)
		m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-pure-guard": true}})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if stats.Funcs[0].Peephole["select-pure-guard"] != 1 {
			t.Fatal("pressure case not admitted")
		}
		for _, args := range [][4]uint64{{3, 17, 0, 1}, {0xffffffff, 19, 9, 2}, {0x80000001, 0xffffffff, 1, 33}} {
			c := uint32(args[2]) / uint32(args[3])
			if predicate == 0x74 {
				c = uint32(args[2]) << (uint32(args[3]) & 31)
			}
			want := uint32(args[1])
			if c != 0 {
				want = ((uint32(args[0]) * 7) ^ 3) * 11
			}
			for i := uint32(1); i <= 24; i++ {
				want ^= uint32(args[0]) * i
			}
			want ^= uint32(args[1])
			if got := runCompiledAmd64u(t, cm, args[:]...); got != uint64(want) {
				t.Fatalf("predicate=%x args=%x got=%x want=%x", predicate, args, got, want)
			}
		}
	}
}
