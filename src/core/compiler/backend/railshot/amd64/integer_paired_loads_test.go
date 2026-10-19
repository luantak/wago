//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestIntegerPairedLoads(t *testing.T) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	for _, firstOffset := range []byte{0, 8, 12} {
		m := integerMemoryModule(t, 8)
		body := append([]byte{}, m.Code[0].BodyBytes...)
		first := bytes.Index(body, []byte{0x20, 0, 0x20, 3, 0x6a, 0x28, 2, 0})
		second := bytes.Index(body, []byte{0x20, 1, 0x20, 3, 0x6a, 0x28, 2, 4})
		body[first+7] = firstOffset
		body[second+1] = 0
		m.Code[0].BodyBytes = body
		init := func(mem []byte) {
			for i := range mem {
				mem[i] = byte(i*37 + 19)
			}
		}
		for _, wide := range []bool{false, true} {
			features := shared.AMD64ModernBaseline
			if wide {
				features |= shared.AMD64AVX2
			}
			for _, n := range []uint64{0, 1, 4, 7, 8, 9, 16, 17, 33} {
				for _, base := range []uint64{0, 1, 65500, 65532, 0xfffffffc} {
					integerMemoryAssertFast = false
					want, _, wantErr := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-memory-vector": false}}, init, base, 0, n*8, 0)
					integerMemoryAssertFast = n >= 8 && base <= 1
					var stats ModuleStats
					got, _, gotErr := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Stats: &stats, Optimizations: map[string]bool{"integer-memory-vector": true, "integer-paired-loads": true}}, init, base, 0, n*8, 0)
					if (wantErr != nil) != (gotErr != nil) || wantErr == nil && got != want {
						t.Fatalf("wide=%v offset=%d n=%d base=%x got=%x/%v want=%x/%v", wide, firstOffset, n, base, got, gotErr, want, wantErr)
					}
					selected := stats.Funcs[0].Peephole["integer-paired-loads"]
					expected := 0
					if wide && firstOffset != 12 {
						expected = 1
					}
					if selected != expected {
						t.Fatalf("paired loads=%d want=%d", selected, expected)
					}
				}
			}
		}
	}
}
