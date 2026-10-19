//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestBMI1AndNot(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		ty, constant, xor, and := wasm.I32, byte(0x41), byte(0x73), byte(0x71)
		if wide {
			ty, constant, xor, and = wasm.I64, 0x42, 0x85, 0x83
		}
		for _, reverse := range []bool{false, true} {
			body := []byte{0}
			if reverse {
				body = append(body, 0x20, 1)
			}
			body = append(body, 0x20, 0, constant, 0x7f, xor)
			if !reverse {
				body = append(body, 0x20, 1)
			}
			body = append(body, and, 0x22, 0, 0x0b)
			m := mod1(t, []wasm.ValType{ty, ty}, []wasm.ValType{ty}, body)
			for _, features := range []shared.AMD64Features{0, shared.AMD64BMI1} {
				for _, enabled := range []bool{false, true} {
					cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Optimizations: map[string]bool{"bmi1-andnot": enabled}})
					if err != nil {
						t.Fatal(err)
					}
					defer cm.CodeImage.Close()
					selected := enabled && features.Has(shared.AMD64BMI1)
					if shared.AMD64Features(cm.RequiredAMD64Features).Has(shared.AMD64BMI1) != selected {
						t.Fatalf("selection enabled=%v features=%x required=%x", enabled, features, cm.RequiredAMD64Features)
					}
					if selected && !cpu.X86.HasBMI1 {
						continue
					}
					for _, x := range []uint64{0, 1, 0xffffffff, 0x8000000000000000, 0xffffffffffffffff, 0x123456789abcdef0} {
						for _, y := range []uint64{0, 1, 0xffffffff, 0xffffffffffffffff, 0xfedcba9876543210} {
							want := ^x & y
							if !wide {
								want = uint64(uint32(want))
							}
							if got := runCompiledAmd64u(t, cm, x, y); got != want {
								t.Fatalf("wide=%v reverse=%v x=%x y=%x got=%x want=%x", wide, reverse, x, y, got, want)
							}
						}
					}
				}
			}
		}
	}
}

func TestBMI1AndNotPressure(t *testing.T) {
	if !cpu.X86.HasBMI1 {
		t.Skip("BMI1 execution unavailable")
	}
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	body := []byte{0}
	for i := byte(1); i <= 24; i++ {
		body = append(body, 0x20, 0, 0x41, i, 0x6c)
	}
	body = append(body, 0x20, 0, 0x41, 0x7f, 0x73, 0x20, 1, 0x71)
	for i := 0; i < 24; i++ {
		body = append(body, 0x73)
	}
	body = append(body, 0x0b)
	m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, enabled := range []bool{false, true} {
		cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64BMI1, Optimizations: map[string]bool{"bmi1-andnot": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		for _, x := range []uint64{0, 1, 0xffffffff, 0x87654321} {
			want := ^uint32(x) & 0xabcdef01
			for i := uint32(1); i <= 24; i++ {
				want ^= uint32(x) * i
			}
			if got := runCompiledAmd64u(t, cm, x, 0xabcdef01); got != uint64(want) {
				t.Fatalf("enabled=%v x=%x got=%x want=%x", enabled, x, got, want)
			}
		}
	}
}
