//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestBMI1ResetLowestBit(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		ty, constant, sub, and, xor := wasm.I32, byte(0x41), byte(0x6b), byte(0x71), byte(0x73)
		if wide {
			ty, constant, sub, and, xor = wasm.I64, 0x42, 0x7d, 0x83, 0x85
		}
		for _, reverse := range []bool{false, true} {
			body := []byte{0}
			if reverse {
				body = append(body, 0x20, 0)
			}
			body = append(body, 0x20, 0, constant, 1, sub)
			if !reverse {
				body = append(body, 0x20, 0)
			}
			body = append(body, and, 0x20, 0, xor, 0x0b)
			m := mod1(t, []wasm.ValType{ty}, []wasm.ValType{ty}, body)
			for _, features := range []shared.AMD64Features{0, shared.AMD64BMI1} {
				for _, on := range []bool{false, true} {
					var stats ModuleStats
					cm, err := CompileModuleWith(m, CompileOptions{Stats: optionalTestStats(&stats), AMD64FeaturesSet: true, AMD64Features: features, Optimizations: map[string]bool{"bmi1-reset-lowest-bit": on}})
					if err != nil {
						t.Fatal(err)
					}
					if cm.CodeImage != nil {
						defer cm.CodeImage.Close()
					}
					selected := on && features.Has(shared.AMD64BMI1)
					if shared.AMD64Features(cm.RequiredAMD64Features).Has(shared.AMD64BMI1) != selected {
						t.Fatalf("required features=%x selected=%v", cm.RequiredAMD64Features, selected)
					}
					if diagnosticsEnabled && (stats.Funcs[0].Peephole["bmi1-reset-lowest-bit"] > 0) != selected {
						t.Fatalf("on=%v features=%x stats=%v", on, features, stats.Funcs[0].Peephole)
					}
					if selected && !cpu.X86.HasBMI1 {
						continue
					}
					for _, x := range []uint64{0, 1, 2, 3, 7, 8, 0x80000000, 0xffffffff, 0x8000000000000000, ^uint64(0), 0x123456789abcdef0} {
						want := (x & (x - 1)) ^ x
						if !wide {
							want = uint64(uint32(want))
						}
						if got := runCompiledAmd64u(t, cm, x); got != want {
							t.Fatalf("wide=%v reverse=%v on=%v x=%x got=%x want=%x", wide, reverse, on, x, got, want)
						}
					}
				}
			}
		}
	}
}

func TestBMI1ResetLowestBitIdentity(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		ty, constant, add, sub, and := wasm.I32, byte(0x41), byte(0x6a), byte(0x6b), byte(0x71)
		if wide {
			ty, constant, add, sub, and = wasm.I64, 0x42, 0x7c, 0x7d, 0x83
		}
		for _, shape := range []string{"add", "constant-left", "different", "overwrite"} {
			body := []byte{0}
			if shape == "constant-left" {
				body = append(body, constant, 0x7f, 0x20, 0, add)
			} else if shape == "add" {
				body = append(body, 0x20, 0, constant, 0x7f, add)
			} else {
				body = append(body, 0x20, 0, constant, 1, sub)
			}
			if shape == "different" {
				body = append(body, 0x20, 1)
			} else if shape == "overwrite" {
				body = append(body, 0x20, 1, 0x22, 0)
			} else {
				body = append(body, 0x20, 0)
			}
			body = append(body, and, 0x0b)
			m := mod1(t, []wasm.ValType{ty, ty}, []wasm.ValType{ty}, body)
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: optionalTestStats(&stats), AMD64FeaturesSet: true, AMD64Features: shared.AMD64BMI1, Optimizations: map[string]bool{"bmi1-reset-lowest-bit": true, "owned-pinned-tee": true}})
			if err != nil {
				t.Fatal(err)
			}
			if cm.CodeImage != nil {
				defer cm.CodeImage.Close()
			}
			selected := shape == "add" || shape == "constant-left"
			if diagnosticsEnabled && (stats.Funcs[0].Peephole["bmi1-reset-lowest-bit"] > 0) != selected {
				t.Fatalf("wide=%v shape=%s stats=%v", wide, shape, stats.Funcs[0].Peephole)
			}
			if shared.AMD64Features(cm.RequiredAMD64Features).Has(shared.AMD64BMI1) != selected {
				t.Fatalf("wide=%v shape=%s required=%x", wide, shape, cm.RequiredAMD64Features)
			}
			if selected && !cpu.X86.HasBMI1 {
				continue
			}
			for _, x := range []uint64{0, 1, 3, 0x80000000, ^uint64(0), 0xfedcba9876543210} {
				for _, y := range []uint64{0, 1, 2, 0x80000000, 0x123456789abcdef0} {
					right := x
					if !selected {
						right = y
					}
					want := (x - 1) & right
					if !wide {
						want = uint64(uint32(want))
					}
					if got := runCompiledAmd64u(t, cm, x, y); got != want {
						t.Fatalf("wide=%v shape=%s x=%x y=%x got=%x want=%x", wide, shape, x, y, got, want)
					}
				}
			}
		}
	}
}

func TestBMI1ResetLowestBitPressure(t *testing.T) {
	if !cpu.X86.HasBMI1 {
		t.Skip("BMI1 execution unavailable")
	}
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	// Keep the bit result live across eager selects and real spill pressure.
	body := []byte{1, 1, 0x7f, 0x20, 0, 0x41, 0x7f, 0x6a, 0x20, 0, 0x71, 0x22, 3}
	for i := byte(1); i <= 24; i++ {
		body = append(body, 0x20, 0, 0x41, i, 0x6a, 0x20, 1, 0x41, i+1, 0x6a, 0x20, 2, 0x1b)
	}
	for range 24 {
		body = append(body, 0x73)
	}
	body = append(body, 0x0b)
	m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, on := range []bool{false, true} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: optionalTestStats(&stats), AMD64FeaturesSet: true, AMD64Features: shared.AMD64BMI1, Optimizations: map[string]bool{"bmi1-reset-lowest-bit": on}})
		if err != nil {
			t.Fatal(err)
		}
		if cm.CodeImage != nil {
			defer cm.CodeImage.Close()
		}
		if diagnosticsEnabled && (stats.Funcs[0].Spills == 0 || (stats.Funcs[0].Peephole["bmi1-reset-lowest-bit"] > 0) != on) {
			t.Fatalf("on=%v stats=%+v", on, stats.Funcs[0])
		}
		for _, x := range []uint64{0, 1, 2, 3, 0x80000000, 0xffffffff, 0x123456789abcdef0} {
			for _, condition := range []uint64{0, 1, 2, 0xffffffff} {
				want := uint32(x) & (uint32(x) - 1)
				for i := uint32(1); i <= 24; i++ {
					if condition != 0 {
						want ^= uint32(x) + i
					} else {
						want ^= 0x89abcdef + i + 1
					}
				}
				if got := runCompiledAmd64u(t, cm, x, 0x89abcdef, condition); got != uint64(want) {
					t.Fatalf("on=%v x=%x condition=%x got=%x want=%x", on, x, condition, got, want)
				}
			}
		}
	}
}

func TestBMI1ResetLowestBitLoop(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	// Ordinary local writeback plus a zero branch: BLSR must remain an integer
	// expression selection, with the source loop and every counter update intact.
	body := []byte{1, 1, 0x7f, 0x02, 0x40, 0x20, 0, 0x45, 0x0d, 0, 0x03, 0x40, 0x20, 1, 0x41, 1, 0x6a, 0x21, 1, 0x20, 0, 0x41, 0x7f, 0x6a, 0x20, 0, 0x71, 0x22, 0, 0x0d, 0, 0x0b, 0x0b, 0x20, 1, 0x0b}
	m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, on := range []bool{false, true} {
		features := shared.AMD64Features(0)
		if cpu.X86.HasBMI1 {
			features = shared.AMD64BMI1
		}
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: optionalTestStats(&stats), AMD64FeaturesSet: true, AMD64Features: features, Optimizations: map[string]bool{"bmi1-reset-lowest-bit": on, "owned-pinned-tee": true}})
		if err != nil {
			t.Fatal(err)
		}
		if cm.CodeImage != nil {
			defer cm.CodeImage.Close()
		}
		if diagnosticsEnabled && (stats.Funcs[0].Peephole["bmi1-reset-lowest-bit"] > 0) != (on && cpu.X86.HasBMI1) {
			t.Fatalf("loop on=%v stats=%v", on, stats.Funcs[0].Peephole)
		}
		for _, x := range []uint64{0, 1, 2, 3, 7, 0x80000000, 0xffffffff, 0x123456789abcdef0} {
			var want uint64
			for value := uint32(x); value != 0; value &= value - 1 {
				want++
			}
			if got := runCompiledAmd64u(t, cm, x); got != want {
				t.Fatalf("on=%v x=%x got=%d want=%d", on, x, got, want)
			}
		}
	}
}

func TestBMI1ResetLowestBitConditions(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		ty, decl, constant, sub, and, first := wasm.I32, byte(0x7f), byte(0x41), byte(0x6b), byte(0x71), byte(0x46)
		if wide {
			ty, decl, constant, sub, and, first = wasm.I64, 0x7e, 0x42, 0x7d, 0x83, 0x51
		}
		for relation := byte(0); relation < 10; relation++ {
			body := []byte{1, 1, decl, 0x20, 0, constant, 1, sub, 0x20, 0, and, 0x22, 1, constant, 0, first + relation, 0x0b}
			m := mod1(t, []wasm.ValType{ty}, []wasm.ValType{wasm.I32}, body)
			for _, on := range []bool{false, true} {
				features := shared.AMD64Features(0)
				if cpu.X86.HasBMI1 {
					features = shared.AMD64BMI1
				}
				cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Optimizations: map[string]bool{"bmi1-reset-lowest-bit": on}})
				if err != nil {
					t.Fatal(err)
				}
				if cm.CodeImage != nil {
					defer cm.CodeImage.Close()
				}
				for _, x := range []uint64{0, 1, 3, 0x80000001, 0xffffffff, 0x8000000000000001, ^uint64(0), 0x123400000000} {
					value := x & (x - 1)
					negative := int64(value) < 0
					if !wide {
						value = uint64(uint32(value))
						negative = int32(value) < 0
					}
					zero := value == 0
					predicates := [10]bool{zero, !zero, negative, false, !negative && !zero, !zero, negative || zero, zero, !negative, true}
					var want uint64
					if predicates[relation] {
						want = 1
					}
					if got := runCompiledAmd64u(t, cm, x); got != want {
						t.Fatalf("wide=%v relation=%v on=%v x=%x got=%d want=%d", wide, relation, on, x, got, want)
					}
				}
			}
		}
	}
}
