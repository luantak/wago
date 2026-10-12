//go:build (linux || darwin || windows) && arm64

package arm64

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

func BenchmarkDirectMap32ARM64(b *testing.B) {
	for _, fp := range []bool{false, true} {
		for _, scalar := range []bool{false, true} {
			for _, n := range []uint64{0, 16, 17, 64, 513, 8192} {
				b.Run(fmt.Sprintf("fp=%v/scalar=%v/n=%d", fp, scalar, n), func(b *testing.B) {
					cm, err := CompileModuleWith(directMap32Fixture(b, fp, scalar), CompileOptions{})
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
					for i, v := range []uint64{0, 32768, 0, n, scale, bias} {
						binary.LittleEndian.PutUint64(args[i*8:], v)
					}
					for i := uint64(0); i < n; i++ {
						binary.LittleEndian.PutUint32(jm.LinearMemory()[32768+4*i:], uint32(i)+0x3f800001)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if err := eng.Call(entry+uintptr(cm.Entry[0]), args, jm.LinearMemory(), trap, results); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func BenchmarkCompileDirectMap32ARM64(b *testing.B) {
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
