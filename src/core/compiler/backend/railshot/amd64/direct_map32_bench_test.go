//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

func BenchmarkDirectMap32AMD64(b *testing.B) {
	for _, fp := range []bool{false, true} {
		for _, scalar := range []bool{false, true} {
			for _, n := range []uint64{0, 4, 8, 16, 17, 64, 513, 8192} {
				b.Run(fmt.Sprintf("fp=%v/scalar=%v/n=%d", fp, scalar, n), func(b *testing.B) {
					benchmarkDirectMap32(b, directMap32Fixture(b, fp, scalar), fp, n)
				})
			}
		}
	}
}

func BenchmarkCompileDirectMap32AMD64(b *testing.B) {
	for _, fp := range []bool{false, true} {
		for _, heap := range []bool{false, true} {
			b.Run(fmt.Sprintf("fp=%v/heap=%v", fp, heap), func(b *testing.B) {
				m := directMap32Fixture(b, fp, false)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					cm, err := CompileModuleWith(m, CompileOptions{Workers: 1, DeferCodeMapping: heap})
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
}

func benchmarkDirectMap32(b *testing.B, m *wasm.Module, fp bool, n uint64) {
	benchmarkDirectMap32WithOptions(b, m, fp, n, CompileOptions{})
}
func benchmarkDirectMap32WithOptions(b *testing.B, m *wasm.Module, fp bool, n uint64, opts CompileOptions) {
	benchmarkDirectMap32At(b, m, fp, n, 0, 32768, opts)
}
func benchmarkDirectMap32At(b *testing.B, m *wasm.Module, fp bool, n, dst, src uint64, opts CompileOptions) {
	cm, err := CompileModuleWith(m, opts)
	if err != nil {
		b.Fatal(err)
	}
	if cm.CodeImage != nil {
		defer cm.CodeImage.Close()
	}
	eng, err := coreruntime.NewEngine()
	if err != nil {
		b.Fatal(err)
	}
	defer eng.Close()
	jm, err := coreruntime.NewJobMemory(65536)
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
	args, results, trap := ar.Alloc(48), ar.Alloc(8), ar.Alloc(coreruntime.TrapBufferBytes)
	scale, bias := uint64(3), uint64(7)
	if fp {
		scale, bias = uint64(math.Float32bits(1.0000001)), uint64(math.Float32bits(-1))
	}
	for i, v := range []uint64{dst, src, 0, n, scale, bias} {
		binary.LittleEndian.PutUint64(args[i*8:], v)
	}
	for i := uint64(0); i < n; i++ {
		binary.LittleEndian.PutUint32(jm.LinearMemory()[src+4*i:], uint32(i)+0x3f800001)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := eng.Call(entry+uintptr(cm.Entry[0]), args, jm.LinearMemory(), trap, results); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileDirectMapF64ControlAMD64(b *testing.B) {
	saved := regionLoopEnabled
	regionLoopEnabled = true
	defer func() { regionLoopEnabled = saved }()
	body := []byte{1, 1, 0x7c, 0x03, 0x40,
		0x20, 0, 0x20, 1, 0x2b, 0, 0, 0x20, 4, 0xa2, 0x20, 5, 0xa0, 0x22, 6, 0x39, 0, 0,
		0x20, 0, 0x41, 8, 0x6a, 0x21, 0, 0x20, 1, 0x41, 8, 0x6a, 0x21, 1,
		0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x20, 3, 0x47, 0x0d, 0, 0x0b, 0x20, 6, 0xbd, 0x0b}
	m := modMem(b, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.F64, wasm.F64}, []wasm.ValType{wasm.I64}, body)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm, err := CompileModuleWith(m, CompileOptions{Workers: 1})
		if err != nil {
			b.Fatal(err)
		}
		if cm.CodeImage != nil {
			cm.CodeImage.Close()
		}
	}
}

func BenchmarkDirectMap32AVX2AMD64(b *testing.B) {
	for _, fp := range []bool{false, true} {
		for _, n := range []uint64{0, 4, 8, 16, 17, 64, 513, 8192} {
			b.Run(fmt.Sprintf("fp=%v/n=%d", fp, n), func(b *testing.B) {
				benchmarkDirectMap32WithOptions(b, directMap32Fixture(b, fp, false), fp, n, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2})
			})
		}
	}
}

func BenchmarkDirectMap32ConstantAMD64(b *testing.B) {
	for _, wide := range []bool{false, true} {
		features := shared.AMD64ModernBaseline
		if wide {
			features |= shared.AMD64AVX2
		}
		for _, scale := range []uint32{0, 1, 2, 3, 256, 1 << 31} {
			b.Run(fmt.Sprintf("avx2=%v/scale=%x", wide, scale), func(b *testing.B) {
				benchmarkDirectMap32WithOptions(b, directMap32ConstantScale(b, false, scale), false, 8192, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features})
			})
		}
	}
}

func BenchmarkDirectMap32ZeroOverlapAMD64(b *testing.B) {
	for _, n := range []uint64{513, 8192} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			benchmarkDirectMap32At(b, directMap32ConstantScale(b, false, 0), false, n, 4, 0, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2})
		})
	}
}
