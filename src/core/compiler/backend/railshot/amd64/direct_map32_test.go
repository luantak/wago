//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
	"github.com/wago-org/wago/tests/support/wasmtest"
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
		for _, features := range []shared.AMD64Features{0, shared.AMD64SSE41, shared.AMD64ModernBaseline} {
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
		for _, features := range []shared.AMD64Features{0, shared.AMD64SSE41, shared.AMD64ModernBaseline} {
			for _, n := range []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 17, 31, 32, 33, 513, 8192} {
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

func BenchmarkDirectMap32Compile(b *testing.B) {
	for _, fp := range []bool{false, true} {
		b.Run(fmt.Sprint(fp), func(b *testing.B) {
			m := directMap32Fixture(b, fp, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline})
				if err != nil {
					b.Fatal(err)
				}
				if cm.CodeImage != nil {
					cm.CodeImage.Close()
				}
			}
		})
	}
	for _, name := range []string{"small", "control", "alu", "simd", "rejected", "corpus-asm", "existing-f64", "yosys"} {
		b.Run(name, func(b *testing.B) {
			var m *wasm.Module
			switch name {
			case "small":
				m = benchSmallScalarModule(b)
			case "control":
				m = benchMediumControlModule(b)
			case "alu":
				m = benchALUHeavyModule(b)
			case "simd":
				m = benchSIMDHeavyModule(b)
			case "rejected":
				m = directMap32Fixture(b, false, true)
			case "corpus-asm", "existing-f64", "yosys":
				root := os.Getenv("DIRECT_MAP32_ROOT")
				if root == "" {
					b.Skip("corpus root not set")
				}
				paths := map[string]string{"corpus-asm": "corpus/workloads/assemblyscript/blake-as-simd.wasm", "existing-f64": "corpus/workloads/polybench/jacobi-2d.wasm", "yosys": "corpus/workloads/applications/yosys/yosys.wasm"}
				raw, err := os.ReadFile(filepath.Join(root, paths[name]))
				if err != nil {
					b.Fatal(err)
				}
				m, err = wasm.DecodeModule(raw)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline})
				if err != nil {
					b.Fatal(err)
				}
				cm.CodeImage.Close()
			}
		})
	}
}

func BenchmarkDirectMap32Execute(b *testing.B) {
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(b, fp, false)
		cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline})
		if err != nil {
			b.Fatal(err)
		}
		defer cm.CodeImage.Close()
		eng, err := coreruntime.NewEngine()
		if err != nil {
			b.Fatal(err)
		}
		defer eng.Close()
		jm, err := coreruntime.NewJobMemory(1 << 25)
		if err != nil {
			b.Fatal(err)
		}
		defer jm.Close()
		ar, err := coreruntime.NewArena(4096)
		if err != nil {
			b.Fatal(err)
		}
		defer ar.Close()
		code, entry, err := coreruntime.MapCode(cm.Code)
		if err != nil {
			b.Fatal(err)
		}
		defer coreruntime.Unmap(code)
		args, out, trap := ar.Alloc(48), ar.Alloc(8), ar.Alloc(coreruntime.TrapBufferBytes)
		for _, n := range []uint32{0, 1, 3, 4, 7, 8, 9, 15, 16, 17, 31, 32, 33, 64, 128, 256, 512, 8192, 262144, 4194304} {
			b.Run(fmt.Sprintf("%v/%d", fp, n), func(b *testing.B) {
				src := uint64(32768)
				if n > 8192 {
					src = uint64(n) * 4
				}
				vals := []uint64{0, src, 0, uint64(n), 3, 7}
				v := uint32(1)
				if fp {
					v = math.Float32bits(1)
				}
				for i := uint32(0); i < n; i++ {
					binary.LittleEndian.PutUint32(jm.CurrentBytes()[src+uint64(i)*4:], v)
				}
				if fp {
					vals[4], vals[5] = f32b(3), f32b(7)
				}
				for i, v := range vals {
					binary.LittleEndian.PutUint64(args[i*8:], v)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := eng.Call(entry+uintptr(cm.Entry[0]), args, jm.LinearMemory(), trap, out); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				want := uint32(n*4) ^ uint32(src+uint64(n)*4) ^ n
				if n != 0 {
					last := uint32(10)
					if fp {
						last = math.Float32bits(10)
					}
					want ^= last
					mem := jm.CurrentBytes()
					if binary.LittleEndian.Uint32(mem) != last || binary.LittleEndian.Uint32(mem[uint64(n-1)*4:]) != last {
						b.Fatal("streaming output mismatch", fp, n)
					}
				}
				if binary.LittleEndian.Uint64(out) != uint64(want) {
					b.Fatal("streaming local mismatch", fp, n, binary.LittleEndian.Uint64(out), want)
				}
			})
		}
	}
}

// Public compilation uses heap staging before the first executable mapping.
// Measure it separately: mapped native bytes do not appear in Go B/op.
func BenchmarkDirectMap32CompileHeap(b *testing.B) {
	for _, name := range []string{"i32", "f32", "rejected"} {
		b.Run(name, func(b *testing.B) {
			m := directMap32Fixture(b, name == "f32", name == "rejected")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, DeferCodeMapping: true})
				if err != nil {
					b.Fatal(err)
				}
				if cm.CodeImage != nil {
					cm.CodeImage.Close()
				}
			}
		})
	}
}

func TestDirectMap32Artifacts(t *testing.T) {
	dir := os.Getenv("DIRECT_MAP32_ARTIFACTS")
	if dir == "" {
		t.Skip("artifact destination not set")
	}
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		ft, _ := m.LocalFuncType(0)
		typ := byte(0x7f)
		if fp {
			typ = 0x7d
		}
		full := append([]byte{1, 1, typ}, m.Code[0].BodyBytes...)
		raw := wasmtest.Module(wasmtest.Section(1, wasmtest.Vec(wasmtest.FuncType(ft.Params, ft.Results))), wasmtest.Section(3, wasmtest.Vec([]byte{0})), wasmtest.Section(5, wasmtest.Vec([]byte{0, 1})), wasmtest.Section(7, wasmtest.Vec(wasmtest.ExportEntry("f", 0, 0))), wasmtest.Section(10, wasmtest.Vec(append(wasmtest.ULEB(uint32(len(full))), full...))))
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("map-%v.wasm", fp)), raw, 0644); err != nil {
			t.Fatal(err)
		}
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: optionalTestStats(&stats)})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("map-%v.bin", fp)), cm.Code, 0644); err != nil {
			t.Fatal(err)
		}
		if diagnosticsEnabled {
			x := stats.Funcs[0]
			t.Logf("fp=%v module=%d function=%d frame=%d spills=%d reloads=%d selected=%d", fp, len(cm.Code), x.CodeBytes, x.FrameBytes, x.Spills, x.Reloads, x.Peephole["region-loop-map32"])
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

// Isolate the shared hint scan from native emission in the ALU control.
func BenchmarkDirectMap32ALUHints(b *testing.B) {
	m := benchALUHeavyModule(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h, _, _, err := computeModuleHints(m, 0, 0, nil, false)
		if err != nil {
			b.Fatal(err)
		}
		if len(h) != 1 || h[0].flags.has(hintHasLoop) {
			b.Fatal("invalid ALU hint fixture")
		}
	}
}
