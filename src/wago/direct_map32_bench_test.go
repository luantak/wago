//go:build linux && amd64

package wago

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectMap32PublicFeatureRecord(t *testing.T) {
	f, ok := cachedAMD64CPUFeatures()
	if !ok {
		t.Fatal("CPU feature detection failed")
	}
	t.Logf("host-features=%#x selected-features=%#x", f, selectedAMD64CompileFeatures(f))
}

func BenchmarkDirectMap32WholeProgram(b *testing.B) {
	dir := os.Getenv("DIRECT_MAP32_INPUTS")
	if dir == "" {
		b.Skip("input directory not set")
	}
	for _, fp := range []bool{false, true} {
		raw, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("map-%v.wasm", fp)))
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range []uint64{0, 16, 8192} {
			b.Run(fmt.Sprintf("%v/%d", fp, n), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					cm, err := Compile(nil, raw)
					if err != nil {
						b.Fatal(err)
					}
					in, err := Instantiate(cm, InstantiateOptions{})
					if err != nil {
						cm.Close()
						b.Fatal(err)
					}
					scale, bias := uint64(3), uint64(7)
					if fp {
						scale, bias = uint64(math.Float32bits(3)), uint64(math.Float32bits(7))
					}
					out, err := in.Invoke("f", 0, 32768, 0, n, scale, bias)
					if err != nil || len(out) != 1 {
						in.Close()
						cm.Close()
						b.Fatal(out, err)
					}
					want := uint64(uint32(n*4) ^ uint32(32768+n*4) ^ uint32(n))
					if n != 0 {
						want ^= bias
					}
					if out[0] != want {
						in.Close()
						cm.Close()
						b.Fatal("whole-program output", out[0], want)
					}
					if err := in.Close(); err != nil {
						cm.Close()
						b.Fatal(err)
					}
					if err := cm.Close(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
