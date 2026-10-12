//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

// The loop has a scalar last-value local and three live induction locals.
func directMap32Fixture(t testing.TB, fp, scalar bool) *wasm.Module {
	typ, load, mul, add, store := wasm.I32, byte(0x28), byte(0x6c), byte(0x6a), byte(0x36)
	if fp {
		typ, load, mul, add, store = wasm.F32, 0x2a, 0x94, 0x92, 0x38
	}
	body := []byte{1, 1, map[bool]byte{false: 0x7f, true: 0x7d}[fp], 0x02, 0x40, 0x20, 3, 0x45, 0x0d, 0, 0x03, 0x40}
	if scalar {
		body = append(body, 0x01)
	}
	body = append(body, 0x20, 0, 0x20, 1, load, 0, 0, 0x20, 4, mul, 0x20, 5, add, 0x22, 6, store, 0, 0,
		0x20, 0, 0x41, 4, 0x6a, 0x21, 0, 0x20, 1, 0x41, 4, 0x6a, 0x21, 1,
		0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x20, 3, 0x47, 0x0d, 0, 0x0b, 0x0b,
		0x20, 0, 0x20, 1, 0x73, 0x20, 2, 0x73, 0x20, 6)
	if fp {
		body = append(body, 0xbc)
	}
	body = append(body, 0x73, 0x0b)
	return modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, typ, typ}, []wasm.ValType{wasm.I32}, body)
}

func TestDirectMap32Selection(t *testing.T) {
	if !diagnosticsEnabled {
		t.Skip("requires wago_codegenstats")
	}
	for _, fp := range []bool{false, true} {
		for _, features := range []shared.AMD64Features{0, shared.AMD64SSE41, shared.AMD64AVX, shared.AMD64ModernBaseline, shared.AMD64ModernBaseline | shared.AMD64AVX2} {
			var stats ModuleStats
			cm, err := CompileModuleWith(directMap32Fixture(t, fp, false), CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Stats: &stats})
			if err != nil {
				t.Fatal(err)
			}
			cm.CodeImage.Close()
			want := fp || features&shared.AMD64SSE41 != 0
			if got := stats.Funcs[0].Peephole["region-loop-map32"] != 0; got != want {
				t.Fatalf("fp=%v features=%d SIMD=%v want=%v stats=%v", fp, features, got, want, stats.Funcs[0].Peephole)
			}
		}
	}
}

func TestDirectMap32Semantics(t *testing.T) {
	for _, fp := range []bool{false, true} {
		ref, candidate := directMap32Fixture(t, fp, true), directMap32Fixture(t, fp, false)
		for _, features := range []shared.AMD64Features{0, shared.AMD64SSE41, shared.AMD64AVX, shared.AMD64ModernBaseline, shared.AMD64ModernBaseline | shared.AMD64AVX2} {
			for _, n := range []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 17, 20, 24, 28, 31, 32, 33, 36, 40, 44, 48, 52, 56, 60, 63, 64, 65, 127, 128, 129, 513, 8192} {
				for _, pair := range [][2]uint64{{128, 32768}, {129, 32769}, {128, 128}, {129, 128}, {128, 129}, {132, 128}, {128, 132}, {128, 65520}, {65520, 128}} {
					setup := func(mem []byte) {
						for i := range mem {
							mem[i] = 0xa5
						}
						values := [...]uint32{0, 0x80000000, 1, 0x7f800000, 0xff800000, 0x7fc12345, 0x3f800001, 0x3f7fffff, 0xffffffff, 0x80000001}
						for i := uint64(0); i < n && pair[1]+4*i+4 <= 65536; i++ {
							binary.LittleEndian.PutUint32(mem[pair[1]+4*i:], values[i%uint64(len(values))])
						}
					}
					scale, bias := uint64(0xffffffff), uint64(0x80000001)
					if fp {
						scale, bias = uint64(math.Float32bits(1.0000001)), uint64(math.Float32bits(-1))
					}
					opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: features}
					a, am, ae := runMemAmd64WithOptions(t, ref, opts, setup, pair[0], pair[1], 0, n, scale, bias)
					b, bm, be := runMemAmd64WithOptions(t, candidate, opts, setup, pair[0], pair[1], 0, n, scale, bias)
					if (ae == nil) != (be == nil) || ae == nil && a != b || !bytes.Equal(am, bm) {
						t.Fatalf("fp=%v features=%d n=%d pair=%v results=%x/%x traps=%v/%v", fp, features, n, pair, a, b, ae, be)
					}
					if ae != nil {
						x, xok := ae.(*coreruntime.TrapError)
						y, yok := be.(*coreruntime.TrapError)
						if !xok || !yok || x.Code != y.Code || len(x.Frames) != len(y.Frames) {
							t.Fatal(ae, be)
						}
						for i := range x.Frames {
							if x.Frames[i].HasProgramCounter != y.Frames[i].HasProgramCounter || x.Frames[i].HasProgramCounter && x.Frames[i].ProgramCounter != y.Frames[i].ProgramCounter+1 {
								t.Fatal("source trap position changed", x.Frames, y.Frames)
							}
						}
					}
				}
			}
		}
	}
}

func directMap32Countdown(t testing.TB, fp, scalar bool) *wasm.Module {
	m := directMap32Fixture(t, fp, scalar)
	body := m.Code[0].BodyBytes
	// Place the original zero test in the loop and decrement its count.
	outer := []byte{0x02, 0x40, 0x20, 3, 0x45, 0x0d, 0, 0x03, 0x40}
	header := []byte{0x02, 0x40, 0x03, 0x40, 0x20, 3, 0x45, 0x0d, 1}
	body = bytes.Replace(body, outer, header, 1)
	latch := []byte{0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x20, 3, 0x47, 0x0d, 0}
	body = bytes.Replace(body, latch, []byte{0x20, 3, 0x41, 1, 0x6b, 0x21, 3, 0x0c, 0}, 1)
	m.Code[0].BodyBytes = body
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	return m
}
func TestDirectMap32Countdown(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, scalar := range []bool{false, true} {
			var stats ModuleStats
			m := directMap32Countdown(t, fp, scalar)
			cm, err := CompileModuleWith(m, CompileOptions{Stats: optionalTestStats(&stats)})
			if err != nil {
				t.Fatal(err)
			}
			cm.CodeImage.Close()
			if diagnosticsEnabled && !scalar && stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
				t.Fatal("countdown map not selected", stats.Funcs[0].Peephole)
			}
		}
		for _, n := range []uint64{0, 1, 4, 8, 16, 17, 32, 33, 8192} {
			opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}
			setup := func(mem []byte) {
				for i := uint64(0); i < n; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)|0x3f800000)
				}
			}
			scale, bias := uint64(3), uint64(7)
			if fp {
				scale, bias = f32b(1.0000001), f32b(-1)
			}
			a, am, ae := runMemAmd64WithOptions(t, directMap32Countdown(t, fp, true), opts, setup, 0, 32768, 0, n, scale, bias)
			b, bm, be := runMemAmd64WithOptions(t, directMap32Countdown(t, fp, false), opts, setup, 0, 32768, 0, n, scale, bias)
			if ae != nil || be != nil || a != b || !bytes.Equal(am, bm) {
				t.Fatal(fp, n, a, b, ae, be)
			}
		}
	}
}

func TestDirectMap32LiveFloatParametersAMD64(t *testing.T) {
	// Keep many float parameters live across the loop. Additional vector
	// temporaries must never overwrite their homes or prevent scalar fallback.
	for _, extra := range []int{10, 11, 12} {
		var result uint64
		var memory []byte
		for _, scalar := range []bool{true, false} {
			m := directMap32Fixture(t, true, scalar)
			ft, _ := m.LocalFuncType(0)
			for i := 0; i < extra; i++ {
				ft.Params = append(ft.Params, wasm.F32)
			}
			body := m.Code[0].BodyBytes
			body = bytes.ReplaceAll(body, []byte{0x22, 6}, []byte{0x22, byte(len(ft.Params))})
			body = bytes.ReplaceAll(body, []byte{0x20, 6}, []byte{0x20, byte(len(ft.Params))})
			body = body[:len(body)-1]
			for i := 6; i < len(ft.Params); i++ {
				body = append(body, 0x20, byte(i), 0xbc, 0x73)
			}
			m.Code[0].BodyBytes = append(body, 0x0b)
			if err := wasm.ValidateModule(m); err != nil {
				t.Fatal(err)
			}
			args := []uint64{0, 32768, 0, 65, uint64(math.Float32bits(1.0000001)), uint64(math.Float32bits(-1))}
			for i := 0; i < extra; i++ {
				args = append(args, uint64(math.Float32bits(float32(i)+0.25)))
			}
			setup := func(mem []byte) {
				for i := 0; i < 65; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)+0x3f800001)
				}
			}
			if diagnosticsEnabled && !scalar && extra == 10 {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: &stats})
				if err != nil {
					t.Fatal(err)
				}
				cm.CodeImage.Close()
				if stats.Funcs[0].Peephole["region-loop-map32"] == 0 {
					t.Fatal("three-register map admission was lost under float pressure")
				}
			}
			got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}, setup, args...)
			if err != nil {
				t.Fatal(err)
			}
			if scalar {
				result, memory = got, mem
			} else if got != result || !bytes.Equal(mem, memory) {
				t.Fatalf("extra=%d changed live parameters or memory", extra)
			}
		}
	}
}

func TestDirectMap32AffineEntryValuesAMD64(t *testing.T) {
	for _, shape := range []string{"constants", "sum", "scaled", "addresses"} {
		for _, n := range []uint64{16, 17, 31, 32, 65} {
			var result uint64
			var memory []byte
			for _, scalar := range []bool{true, false} {
				m := directMap32Fixture(t, false, scalar)
				body := m.Code[0].BodyBytes
				switch shape {
				case "constants":
					body = bytes.ReplaceAll(body, []byte{0x20, 4}, []byte{0x41, 3})
					body = bytes.ReplaceAll(body, []byte{0x20, 5}, []byte{0x41, 7})
				case "sum":
					body = bytes.Replace(body, []byte{0x20, 4, 0x6c}, []byte{0x20, 4, 0x20, 5, 0x6a, 0x6c}, 1)
				case "scaled":
					body = bytes.Replace(body, []byte{0x20, 4, 0x6c}, []byte{0x20, 4, 0x41, 3, 0x6c, 0x41, 5, 0x6a, 0x6c}, 1)
				case "addresses":
					body = bytes.Replace(body, []byte{0x20, 0, 0x20, 1, 0x28}, []byte{0x20, 0, 0x41, 16, 0x6a, 0x20, 1, 0x41, 16, 0x6a, 0x28}, 1)
				}
				m.Code[0].BodyBytes = body
				if err := wasm.ValidateModule(m); err != nil {
					t.Fatal(err)
				}
				if !scalar && diagnosticsEnabled {
					var stats ModuleStats
					cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: &stats})
					if err != nil {
						t.Fatal(err)
					}
					if cm.CodeImage != nil {
						cm.CodeImage.Close()
					}
					if stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
						t.Fatalf("shape=%s did not select SIMD", shape)
					}
				}
				setup := func(mem []byte) {
					for i := 0; i < 100; i++ {
						binary.LittleEndian.PutUint32(mem[32769+4*i:], uint32(i)*0x12345+7)
					}
				}
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}, setup, 129, 32769, 7, 7+n, 0xfffffff3, 0x80000001)
				if err != nil {
					t.Fatal(err)
				}
				if scalar {
					result, memory = got, mem
				} else if got != result || !bytes.Equal(mem, memory) {
					t.Fatalf("shape=%s n=%d changed result or memory", shape, n)
				}
			}
		}
	}
}

func TestDirectMap32WideFeatureRequirements(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, features := range []shared.AMD64Features{0, shared.AMD64SSE41, shared.AMD64ModernBaseline, shared.AMD64ModernBaseline | shared.AMD64AVX2} {
			cm, err := CompileModuleWith(directMap32Fixture(t, fp, false), CompileOptions{AMD64FeaturesSet: true, AMD64Features: features})
			if err != nil {
				t.Fatal(err)
			}
			cm.CodeImage.Close()
			if shared.AMD64Features(cm.RequiredAMD64Features) & ^features != 0 {
				t.Fatalf("fp=%v features=%x required=%x", fp, features, cm.RequiredAMD64Features)
			}
			wantAVX2 := !fp && features.Has(shared.AMD64AVX2)
			if cm.RequiresAVX2 != wantAVX2 {
				t.Fatalf("fp=%v features=%x avx2=%v want=%v", fp, features, cm.RequiresAVX2, wantAVX2)
			}
		}
	}
}
