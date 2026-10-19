//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestShortSelectMul(t *testing.T) {
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		typ, mul, cst := wasm.I32, byte(0x6c), byte(0x41)
		if wide {
			typ, mul, cst = wasm.I64, 0x7e, 0x42
		}
		for _, constant := range []bool{false, true} {
			body := []byte{0, 0x20, 0}
			if constant {
				body = append(body, cst, 9)
			} else {
				body = append(body, 0x20, 1)
			}
			body = append(body, mul, cst, 0, 0x20, 2, 0x1b, 0x0b)
			for _, on := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(mod1(t, []wasm.ValType{typ, typ, wasm.I32}, []wasm.ValType{typ}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-pure-guard": true, "cheap-select-guard": true, "short-select-guard": true, "short-select-mul": on}})
				if err != nil {
					t.Fatal(err)
				}
				for _, a := range []uint64{0, 1, 0xffffffff, 0x8000000000000000, ^uint64(0)} {
					for _, b := range []uint64{0, 1, 0x80000000, ^uint64(0)} {
						for _, cond := range []uint64{0, 1, 17} {
							y := b
							if constant {
								y = 9
							}
							want := a * y
							if cond == 0 {
								want = 0
							}
							if !wide {
								want = uint64(uint32(want))
							}
							if got := runCompiledAmd64u(t, cm, a, b, cond); got != want {
								t.Fatalf("wide=%v const=%v on=%v got=%x want=%x", wide, constant, on, got, want)
							}
						}
					}
				}
				if (stats.Funcs[0].Peephole["short-select-guard"] > 0) != on {
					t.Fatal("short multiply guard admission")
				}
				cm.CodeImage.Close()
			}
		}
	}
}
