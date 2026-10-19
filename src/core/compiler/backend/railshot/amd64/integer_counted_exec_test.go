//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestIntegerCountedVectorExecution(t *testing.T) {
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	oldScalar, oldAssert := sharedScalarEnabled, integerMemoryAssertFast
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = oldScalar; integerMemoryAssertFast = oldAssert }()
	for _, shift := range []byte{0x74, 0x76} {
		for _, predicate := range []byte{0x46, 0x47, 0} {
			// Two independent induction variables, a select with oversized shift
			// counts, and a modular reduction. Return every carried local's value.
			body := []byte{0, 0x02, 0x40, 0x20, 0, 0x20, 1, 0x46, 0x0d, 0, 0x03, 0x40,
				0x20, 3, 0x20, 1, 0x41, 0x61, 0x41, 0x7f, 0x20, 2, 0x20, 0, 0x46, 0x1b, shift, 0x6a, 0x21, 3,
				0x20, 2, 0x41, 3, 0x6b, 0x21, 2,
				0x20, 1, 0x41, 2, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b,
				0x0b, 0x20, 3, 0x20, 2, 0x73, 0x20, 1, 0x73, 0x0b}
			pos := bytes.Index(body, []byte{0x20, 2, 0x20, 0, 0x46, 0x1b})
			if predicate == 0 {
				body = append(body[:pos+2], body[pos+5:]...)
			} else {
				body[pos+4] = predicate
			}
			m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
			for _, wide := range []bool{false, true} {
				features := shared.AMD64ModernBaseline
				if wide {
					features |= shared.AMD64AVX2
				}
				for _, force := range []bool{false, true} {
					integerMemoryAssertFast = force && wide
					cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Optimizations: map[string]bool{"integer-counted-vector": true}})
					if err != nil {
						t.Fatal(err)
					}
					if cm.RequiresAVX2 != wide {
						t.Fatal("counted vector path not selected or feature gate failed")
					}
					for n := uint32(0); n <= 41; n++ {
						for _, start := range []uint32{0, 7, 0x80000000, 0xfffffff0} {
							limit := start + 2*n
							if force && wide && (n < 8 || limit <= start) {
								continue
							}
							for _, initial := range []uint32{limit, limit + 9, 0xffffffff} {
								i, aux, sum := start, initial, uint32(0xfffffff1)
								for k := uint32(0); k < n; k++ {
									count := uint32(31)
									condition := aux == limit
									if predicate == 0x47 {
										condition = aux != limit
									} else if predicate == 0 {
										condition = aux != 0
									}
									if condition {
										count = 1
									}
									term := i << count
									if shift == 0x76 {
										term = i >> count
									}
									sum += term
									i += 2
									aux -= 3
								}
								want := uint64(sum ^ aux ^ i)
								if got := runCompiledAmd64u(t, cm, uint64(limit), uint64(start), uint64(initial), 0xfffffff1); got != want {
									t.Fatalf("shift=%x wide=%v force=%v n=%d start=%x aux=%x got=%x want=%x", shift, wide, force, n, start, initial, got, want)
								}
							}
						}
					}
					cm.CodeImage.Close()
				}
			}
		}
	}
}
