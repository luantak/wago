//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestLeaDouble(t *testing.T) {
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		typ, cst, shift, xor, neg := wasm.I32, byte(0x41), byte(0x74), byte(0x73), byte(0x61)
		if wide {
			typ, cst, shift, xor, neg = wasm.I64, 0x42, 0x86, 0x85, 0x41
		}
		for _, count := range []byte{1, neg} {
			m := mod1(t, []wasm.ValType{typ}, []wasm.ValType{typ}, []byte{0, 0x20, 0, cst, count, shift, 0x20, 0, xor, 0x0b})
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"lea-double": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				if (stats.Funcs[0].Peephole["lea-double"] > 0) != enabled {
					t.Fatalf("missing admission wide=%v enabled=%v", wide, enabled)
				}
				for _, x := range []uint64{0, 1, 0x80000000, 0xffffffff, 0x8000000000000000, 0xffffffffffffffff, 0x123456789abcdef0} {
					want := (x << 1) ^ x
					if !wide {
						want = uint64(uint32(want))
					}
					if got := runCompiledAmd64u(t, cm, x); got != want {
						t.Fatalf("wide=%v enabled=%v x=%x got=%x want=%x", wide, enabled, x, got, want)
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}

func TestLeaDoublePreservesOldLocal(t *testing.T) {
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		typ, cst, shift, xor := wasm.I32, byte(0x41), byte(0x74), byte(0x73)
		if wide {
			typ, cst, shift, xor = wasm.I64, 0x42, 0x86, 0x85
		}
		m := mod1(t, []wasm.ValType{typ, typ}, []wasm.ValType{typ}, []byte{0, 0x20, 0, cst, 1, shift, 0x20, 1, 0x21, 0, 0x20, 0, xor, 0x0b})
		for _, on := range []bool{false, true} {
			cm, err := CompileModuleWith(m, CompileOptions{Optimizations: map[string]bool{"lea-double": on}})
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][2]uint64{{3, 19}, {0xffffffff, 0x80000000}, {0x8000000000000000, 0xffffffffffffffff}} {
				want := (args[0] << 1) ^ args[1]
				if !wide {
					want = uint64(uint32(want))
				}
				if got := runCompiledAmd64u(t, cm, args[0], args[1]); got != want {
					t.Fatalf("wide=%v on=%v args=%x got=%x want=%x", wide, on, args, got, want)
				}
			}
			cm.CodeImage.Close()
		}
	}
}
