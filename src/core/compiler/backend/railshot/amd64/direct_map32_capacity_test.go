//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
)

func TestDirectMap32HeapAllocationBudget(t *testing.T) {
	if profileEnabled {
		t.Skip("allocation budget applies to builds without profile instrumentation")
	}
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

func TestDirectMap32ReserveBounds(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, rejected := range []bool{false, true} {
			m := directMap32Fixture(t, fp, rejected)
			h, err := scanFuncBody(m.Code[0], 7, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			classifier := wasm.NewModuleInstructionClassifier(m, true)
			opts := CompileOptions{AMD64Features: shared.AMD64ModernBaseline}
			reserve := directMap32CodeReserve(m, 0, h.funcHints, opts, classifier)
			want := map32CodeReserve
			if rejected {
				want = 0
			}
			if reserve != want {
				t.Fatalf("fp=%t rejected=%t reserve=%d want=%d", fp, rejected, reserve, want)
			}
			if allocs := testing.AllocsPerRun(1000, func() { directMap32CodeReserve(m, 0, h.funcHints, opts, classifier) }); allocs != 0 {
				t.Fatalf("reserve allocates %g objects", allocs)
			}
			opts.AMD64Features = 0
			if !fp && directMap32CodeReserve(m, 0, h.funcHints, opts, classifier) != 0 {
				t.Fatal("unsupported i32 CPU reserves space")
			}
			opts.AMD64Features = shared.AMD64ModernBaseline
			opts.Interruptible = true
			if directMap32CodeReserve(m, 0, h.funcHints, opts, classifier) != 0 {
				t.Fatal("interruptible function reserves space")
			}
			opts.Interruptible = false
			m.Memories[0].Shared = true
			if directMap32CodeReserve(m, 0, h.funcHints, opts, classifier) != 0 {
				t.Fatal("shared memory reserves space")
			}
			m.Memories[0].Shared = false
			m.Memories[0].Limits.Addr64 = true
			if directMap32CodeReserve(m, 0, h.funcHints, opts, classifier) != 0 {
				t.Fatal("memory64 reserves space")
			}
			m.Memories[0].Limits.Addr64 = false
			m.Code[0].BodyBytes = append(m.Code[0].BodyBytes, make([]byte, regionLoopMaxBytes)...)
			if directMap32CodeReserve(m, 0, h.funcHints, opts, classifier) != 0 {
				t.Fatal("large function exceeds scan bound")
			}
		}
	}
}
