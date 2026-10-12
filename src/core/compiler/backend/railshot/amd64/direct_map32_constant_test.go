//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
	"github.com/wago-org/wago/tests/support/wasmtest"
	"testing"
)

func directMap32ConstantScale(t testing.TB, scalar bool, scale uint32) *wasm.Module {
	t.Helper()
	m := directMap32Fixture(t, false, scalar)
	value := append([]byte{0x41}, wasmtest.SLEB32(int32(scale))...)
	value = append(value, 0x6c)
	m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 4, 0x6c}, value, 1)
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDirectMap32ConstantScaleAMD64(t *testing.T) {
	for _, features := range []shared.AMD64Features{0, shared.AMD64SSE41, shared.AMD64ModernBaseline, shared.AMD64ModernBaseline | shared.AMD64AVX2} {
		opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: features}
		for _, scale := range []uint32{0, 1, 2, 3, 4, 8, 256, 1 << 30, 1 << 31, 0xffffffff} {
			for _, n := range []uint64{0, 1, 15, 16, 17, 31, 32, 33, 65} {
				var result uint64
				var memory []byte
				for _, scalar := range []bool{true, false} {
					m := directMap32ConstantScale(t, scalar, scale)
					if !scalar && diagnosticsEnabled {
						var stats ModuleStats
						cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, AMD64FeaturesSet: true, AMD64Features: features})
						if err != nil {
							t.Fatal(err)
						}
						if cm.CodeImage != nil {
							cm.CodeImage.Close()
						}
						if stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
							t.Fatalf("scale=%x did not select SIMD", scale)
						}
					}
					setup := func(mem []byte) {
						for i := 0; i < 100; i++ {
							binary.LittleEndian.PutUint32(mem[32769+4*i:], uint32(i)*0xabcdef+0x80000001)
						}
					}
					got, mem, err := runMemAmd64WithOptions(t, m, opts, setup, 129, 32769, 0, n, 0, 0x80000001)
					if err != nil {
						t.Fatal(err)
					}
					if scalar {
						result, memory = got, mem
					} else if got != result || !bytes.Equal(mem, memory) {
						t.Fatalf("scale=%x n=%d changed result or memory", scale, n)
					}
				}
			}
		}
	}
}

// Removing zero-scale arithmetic must retain checked traps and scalar fallback.
func TestDirectMap32ZeroScaleBoundsAMD64(t *testing.T) {
	for _, n := range []uint64{16, 17, 65} {
		for _, pair := range [][2]uint64{{129, 32769}, {129, 129}, {129, 125}, {125, 129}, {129, 65520}, {65520, 129}} {
			setup := func(mem []byte) {
				for i := range mem {
					mem[i] = byte(i*17 + 3)
				}
			}
			a, am, ae := runMemAmd64WithOptions(t, directMap32ConstantScale(t, true, 0), CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2}, setup, pair[0], pair[1], 0, n, 0, 0x80000001)
			b, bm, be := runMemAmd64WithOptions(t, directMap32ConstantScale(t, false, 0), CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2}, setup, pair[0], pair[1], 0, n, 0, 0x80000001)
			if (ae == nil) != (be == nil) || ae == nil && a != b || !bytes.Equal(am, bm) {
				t.Fatalf("n=%d pair=%v results=%x/%x errors=%v/%v", n, pair, a, b, ae, be)
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

func TestDirectMap32ConstantMultiplyOnlyAMD64(t *testing.T) {
	for _, scale := range []uint32{0, 1, 2, 1 << 31} {
		for _, n := range []uint64{16, 17, 65} {
			var result uint64
			var memory []byte
			for _, scalar := range []bool{true, false} {
				m := directMap32ConstantScale(t, scalar, scale)
				m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 5, 0x6a}, nil, 1)
				if err := wasm.ValidateModule(m); err != nil {
					t.Fatal(err)
				}
				setup := func(mem []byte) {
					for i := range mem {
						mem[i] = byte(i*17 + 3)
					}
				}
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2}, setup, 129, 32769, 0, n, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				if scalar {
					result, memory = got, mem
				} else if got != result || !bytes.Equal(mem, memory) {
					t.Fatalf("scale=%x n=%d changed result or memory", scale, n)
				}
			}
		}
	}
}
