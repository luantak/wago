//go:build linux && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
)

func TestDirectMap32HeapAllocationBudget(t *testing.T) {
	for _, fp := range []bool{false, true} {
		scalar, candidate := directMap32Fixture(t, fp, true), directMap32Fixture(t, fp, false)
		opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Workers: 1, DeferCodeMapping: true}
		count := func(useCandidate bool) float64 {
			m := scalar
			if useCandidate {
				m = candidate
			}
			return testing.AllocsPerRun(100, func() {
				cm, err := CompileModuleWith(m, opts)
				if err != nil {
					t.Fatal(err)
				}
				if cm.CodeImage != nil {
					cm.CodeImage.Close()
				}
			})
		}
		want, got := count(false), count(true)
		if got != want {
			t.Errorf("fp=%t: heap compile allocations = %g, scalar = %g", fp, got, want)
		}
	}
}

func TestDirectMap32CapacityPreservesCode(t *testing.T) {
	for _, count := range []int{1, 32, 1024} {
		m := directMap32Fixture(t, false, false)
		fn := m.Code[0]
		for len(m.Code) < count {
			m.Code = append(m.Code, fn)
			m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
		}
		for _, workers := range []int{1, 2} {
			opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Workers: workers}
			mapped, err := CompileModuleWith(m, opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.DeferCodeMapping = true
			heap, err := CompileModuleWith(m, opts)
			if err != nil {
				mapped.CodeImage.Close()
				t.Fatal(err)
			}
			if !bytes.Equal(mapped.Code, heap.Code) {
				t.Errorf("functions=%d workers=%d: capacity changed native code", count, workers)
			}
			if mapped.CodeImage != nil {
				mapped.CodeImage.Close()
			}
			if heap.CodeImage != nil {
				heap.CodeImage.Close()
			}
		}
	}
}
