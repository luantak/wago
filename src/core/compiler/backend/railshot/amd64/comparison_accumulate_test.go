//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestComparisonAccumulate(t *testing.T) {
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		typ, add, lt := wasm.I32, byte(0x6a), byte(0x49)
		if wide {
			typ, add, lt = wasm.I64, 0x7c, 0x54
		}
		for _, delta := range []byte{0, 2, 4, 6} {
			for _, sub := range []bool{false, true} {
				body := []byte{0, 0x20, 0, 0x20, 1, 0x20, 2, add, 0x20, 3, lt + delta}
				alu := byte(0x6a)
				if sub {
					alu = 0x6b
				}
				body = append(body, alu, 0x0b)
				var stats ModuleStats
				cm, err := CompileModuleWith(mod1(t, []wasm.ValType{wasm.I32, typ, typ, typ}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"comparison-accumulate": true}})
				if err != nil {
					t.Fatal(err)
				}
				for _, a := range []uint64{0, 1, 0xffffffff, 0x8000000000000000, ^uint64(0)} {
					for _, b := range []uint64{0, 1, 0xffffffff, ^uint64(0)} {
						for _, c := range []uint64{0, 1, 0x80000000, ^uint64(0)} {
							x, y := a+b, c
							if !wide {
								x, y = uint64(uint32(x)), uint64(uint32(y))
							}
							v := false
							switch delta {
							case 0:
								v = x < y
							case 2:
								v = x > y
							case 4:
								v = x <= y
							case 6:
								v = x >= y
							}
							want := uint32(a)
							if v {
								if sub {
									want--
								} else {
									want++
								}
							}
							if got := runCompiledAmd64u(t, cm, a, a, b, c); got != uint64(want) {
								t.Fatalf("wide=%v delta=%d sub=%v args=%x,%x,%x got=%x want=%x", wide, delta, sub, a, b, c, got, want)
							}
						}
					}
				}
				if stats.Funcs[0].Peephole["comparison-accumulate"] != 1 {
					t.Fatal("missing admission")
				}
				cm.CodeImage.Close()
			}
		}
	}
}
