//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestBMI2VariableShift(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		ty, base := wasm.I32, byte(0x74)
		if wide {
			ty, base = wasm.I64, 0x86
		}
		for kind := byte(0); kind < 3; kind++ {
			m := mod1(t, []wasm.ValType{ty, ty}, []wasm.ValType{ty}, []byte{0, 0x20, 0, 0x20, 1, base + kind, 0x0b})
			for _, enabled := range []bool{false, true} {
				cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64BMI2, Optimizations: map[string]bool{"bmi2-shift": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				defer cm.CodeImage.Close()
				if cm.RequiresBMI2 != enabled {
					t.Fatalf("BMI2 requirement=%v enabled=%v", cm.RequiresBMI2, enabled)
				}
				if enabled && !cpu.X86.HasBMI2 {
					continue
				}
				for _, x := range []uint64{0, 1, 0x80000000, 0xffffffff, 0x8000000000000000, 0xffffffffffffffff, 0x123456789abcdef0} {
					for _, n := range []uint64{0, 1, 7, 31, 32, 33, 63, 64, 65, 0xffffffffffffffff} {
						var want uint64
						if wide {
							switch kind {
							case 0:
								want = x << (n & 63)
							case 1:
								want = uint64(int64(x) >> (n & 63))
							case 2:
								want = x >> (n & 63)
							}
						} else {
							switch kind {
							case 0:
								want = uint64(uint32(x) << (n & 31))
							case 1:
								want = uint64(uint32(int32(x) >> (n & 31)))
							case 2:
								want = uint64(uint32(x) >> (n & 31))
							}
						}
						if got := runCompiledAmd64u(t, cm, x, n); got != want {
							t.Fatalf("wide=%v kind=%d x=%x n=%x got=%x want=%x", wide, kind, x, n, got, want)
						}
					}
				}
			}
			cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Optimizations: map[string]bool{"bmi2-shift": true}})
			if err != nil {
				t.Fatal(err)
			}
			if cm.RequiresBMI2 {
				t.Fatal("BMI2 selected without feature")
			}
			cm.CodeImage.Close()
		}
	}
}

func TestBMI2ShiftPressure(t *testing.T) {
	if !cpu.X86.HasBMI2 {
		t.Skip("BMI2 execution unavailable")
	}
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	body := []byte{0}
	for i := byte(1); i <= 24; i++ {
		body = append(body, 0x20, 0, 0x41, i, 0x6c)
	}
	// The count uses division's fixed RAX/RDX roles while the left value and
	// 24 earlier values remain live. This exercises reloads and spill ownership.
	body = append(body, 0x20, 0, 0x20, 1, 0x20, 2, 0x6e, 0x74)
	for i := 0; i < 24; i++ {
		body = append(body, 0x73)
	}
	body = append(body, 0x0b)
	m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, enabled := range []bool{false, true} {
		cm, err := CompileModuleWith(m, CompileOptions{Optimizations: map[string]bool{"bmi2-shift": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if cm.RequiresBMI2 != enabled {
			t.Fatal("pressure selection mismatch")
		}
		for _, a := range [][3]uint64{{1, 65, 2}, {0xffffffff, 127, 3}, {0x87654321, 0xffffffff, 17}, {15, 0, 1}} {
			want := uint32(a[0]) << ((uint32(a[1]) / uint32(a[2])) & 31)
			for i := uint32(1); i <= 24; i++ {
				want ^= uint32(a[0]) * i
			}
			if got := runCompiledAmd64u(t, cm, a[:]...); got != uint64(want) {
				t.Fatalf("enabled=%v args=%x got=%x want=%x", enabled, a, got, want)
			}
		}
	}
	trap := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, []byte{0, 0x20, 0, 0x20, 1, 0x20, 2, 0x6e, 0x74, 0x0b})
	for _, enabled := range []bool{false, true} {
		_, _, err := runMemAmd64WithOptions(t, trap, CompileOptions{Optimizations: map[string]bool{"bmi2-shift": enabled}}, nil, 0, 1, 0)
		if err == nil {
			t.Fatal("division trap skipped")
		}
	}
}
