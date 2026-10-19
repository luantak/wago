//go:build amd64 && !tinygo && !wago_profile

package amd64

import (
	"fmt"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestSharedScalarFrameUsage(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = true
	defer func() { sharedScalarEnabled = saved }()
	for _, tc := range []struct {
		name               string
		body               []byte
		enabled, wantFrame bool
	}{
		{"register-only", []byte{0, 0x20, 0, 0x41, 1, 0x6a, 0x0b}, true, false},
		{"disabled", []byte{0, 0x20, 0, 0x41, 1, 0x6a, 0x0b}, false, true},
		{"control-home", []byte{0, 0x20, 0, 0x04, 0x40, 0x41, 7, 0x21, 0, 0x05, 0x41, 9, 0x21, 0, 0x0b, 0x20, 0, 0x0b}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, tc.body)
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"frame-elide": tc.enabled}})
			if err != nil {
				t.Fatal(err)
			}
			defer cm.CodeImage.Close()
			if !stats.Funcs[0].SharedScalar || (stats.Funcs[0].FrameBytes != 0) != tc.wantFrame {
				t.Fatalf("frame=%d wantFrame=%v", stats.Funcs[0].FrameBytes, tc.wantFrame)
			}
			for _, x := range []uint64{0, 1, 0xffffffff, 0xabcdef0000000000} {
				want := uint32(x) + 1
				if tc.name == "control-home" {
					want = 9
					if uint32(x) != 0 {
						want = 7
					}
				}
				if got := runCompiledAmd64u(t, cm, x); got != uint64(want) {
					t.Fatalf("x=%x got=%x want=%x", x, got, want)
				}
			}
		})
	}
}

func TestSharedScalarIncomingParameters(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = true
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		for count := 1; count <= len(intArgRegs); count++ {
			t.Run(fmt.Sprintf("wide=%v/params=%d", wide, count), func(t *testing.T) {
				typ, add, shift := wasm.I32, byte(0x6a), byte(0x74)
				if wide {
					typ, add, shift = wasm.I64, 0x7c, 0x86
				}
				params := make([]wasm.ValType, count)
				body := []byte{0, 0x20, 0}
				for i := range params {
					params[i] = typ
					// Repeated reads keep incoming values live across a variable
					// shift, including the fixed RCX shift-count register.
					body = append(body, 0x20, byte(i), shift, 0x20, byte(i), add)
				}
				body = append(body, 0x0b)
				m := mod1(t, params, []wasm.ValType{typ}, body)
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Workers: 1})
				if err != nil {
					t.Fatal(err)
				}
				defer cm.CodeImage.Close()
				if !stats.Funcs[0].SharedScalar || stats.Funcs[0].Peephole["scalar-incoming-param"] != count {
					t.Fatalf("incoming parameters not retained: %+v", stats.Funcs[0])
				}
				for _, seed := range []uint64{0, 1, 0xabcdef1280000001, ^uint64(0)} {
					args := make([]uint64, count)
					for i := range args {
						args[i] = seed + uint64(i)
					}
					want := args[0]
					for _, x := range args {
						if wide {
							want = (want << (x & 63)) + x
						} else {
							want = uint64((uint32(want) << (x & 31)) + uint32(x))
						}
					}
					if got := runCompiledAmd64u(t, cm, args...); got != want {
						t.Fatalf("seed=%x got=%x want=%x", seed, got, want)
					}
				}
			})
		}
	}
}
