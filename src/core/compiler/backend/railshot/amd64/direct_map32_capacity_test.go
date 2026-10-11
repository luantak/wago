//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"reflect"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
)

func TestDirectMap32HeapAllocationBudget(t *testing.T) {
	if profileEnabled {
		t.Skip("allocation budget applies to builds without profile instrumentation")
	}
	for _, fp := range []bool{false, true} {
		for _, shape := range []string{"ascending", "countdown", "step2", "step4", "affine", "last-homes"} {
			if fp && shape == "affine" {
				continue
			}
			scalar, candidate := directMap32Fixture(t, fp, true), directMap32Fixture(t, fp, false)
			if shape == "countdown" {
				scalar, candidate = directMap32Countdown(t, fp, true), directMap32Countdown(t, fp, false)
			}
			for _, m := range []*wasm.Module{scalar, candidate} {
				body := m.Code[0].BodyBytes
				if shape == "step2" || shape == "step4" {
					step := byte(2)
					if shape == "step4" {
						step = 4
					}
					body = bytes.Replace(body, []byte{0x20, 2, 0x41, 1, 0x6a}, []byte{0x20, 2, 0x41, step, 0x6a}, 1)
				}
				if shape == "affine" && !fp {
					body = bytes.Replace(body, []byte{0x20, 4, 0x6c}, []byte{0x20, 4, 0x20, 5, 0x6a, 0x6c}, 1)
				}
				if shape == "last-homes" {
					m.Code[0].Locals.Runs[0].Count = 2
					store := byte(0x36)
					if fp {
						store = 0x38
					}
					body = bytes.Replace(body, []byte{0x22, 6, store}, []byte{0x22, 6, 0x22, 7, store}, 1)
				}
				m.Code[0].BodyBytes = body
				if err := wasm.ValidateModule(m); err != nil {
					t.Fatal(err)
				}
			}
			opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Workers: 1, DeferCodeMapping: true}
			ft, _ := candidate.LocalFuncType(0)
			nLocals := len(ft.Params)
			for _, run := range candidate.Code[0].Locals.Runs {
				nLocals += int(run.Count)
			}
			h, err := scanFuncBody(candidate.Code[0], nLocals, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			if directMap32CodeReserve(candidate, 0, h.funcHints, opts, wasm.NewModuleInstructionClassifier(candidate, true)) == 0 {
				t.Fatal("allocation fixture lacks an admitted map", fp, shape)
			}
			if diagnosticsEnabled {
				var stats ModuleStats
				preflight := opts
				preflight.Stats = &stats
				cm, err := CompileModuleWith(candidate, preflight)
				if err != nil {
					t.Fatal(err)
				}
				if cm.CodeImage != nil {
					cm.CodeImage.Close()
				}
				if stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
					t.Fatal("allocation fixture did not emit SIMD", fp, shape)
				}
			}
			count := func(m *wasm.Module) float64 {
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
			want, got := count(scalar), count(candidate)
			if got != want {
				t.Errorf("fp=%t shape=%s: heap compile allocations = %g, scalar = %g", fp, shape, got, want)
			}
		}
	}
}

func TestDirectMap32CapacityPreservesCode(t *testing.T) {
	for _, count := range []int{1, 32, 2048, 2049} {
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
				if mapped.CodeImage != nil {
					mapped.CodeImage.Close()
				}
				t.Fatal(err)
			}
			if !reflect.DeepEqual(mapped.Entry, heap.Entry) || !reflect.DeepEqual(mapped.InternalEntry, heap.InternalEntry) || mapped.RequiredAMD64Features != heap.RequiredAMD64Features || mapped.RequiresBMI2 != heap.RequiresBMI2 || mapped.RequiresBitCount != heap.RequiresBitCount {
				t.Errorf("functions=%d workers=%d: capacity changed entry or CPU metadata", count, workers)
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

func TestDirectMap32FunctionReserveLimit(t *testing.T) {
	m := directMap32Fixture(t, false, false)
	classifier := wasm.NewModuleInstructionClassifier(m, true)
	r := wasm.ReaderFrom(m.Code[0].BodyBytes)
	start, end := 0, 0
	var types = []machineType{mtI32, mtI32, mtI32, mtI32, mtI32, mtI32, mtI32}
	for r.BytesLeft() > 0 {
		pc := r.Offset()
		op, _ := r.Byte()
		var imm wasm.InstructionImmediate
		if classifier.ClassifyInto(&r, op, &imm) != nil {
			t.Fatal("invalid fixture")
		}
		if op == 3 {
			var p regionLoopPlan
			if !inspectRegionLoop(r, types, classifier, &p) {
				t.Fatal("fixture loop rejected")
			}
			start, end = pc, p.endPC
			break
		}
	}
	if end == 0 {
		t.Fatal("fixture has no loop")
	}
	original := m.Code[0].BodyBytes
	loop := original[start:end]
	for _, count := range []int{1, 4, 5} {
		body := append([]byte(nil), original[:start]...)
		for i := 0; i < count; i++ {
			body = append(body, loop...)
		}
		body = append(body, original[end:]...)
		m.Code[0].BodyBytes = body
		h, err := scanFuncBody(m.Code[0], len(types), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		got := directMap32CodeReserve(m, 0, h.funcHints, CompileOptions{AMD64Features: shared.AMD64ModernBaseline}, classifier)
		want := min(count*map32CodeReserve, map32FunctionReserveLimit)
		if got != want {
			t.Errorf("loops=%d reserve=%d want=%d", count, got, want)
		}
	}
}

// Capacity must consume the retained hint, not read the function again.
func TestDirectMap32CapacityUsesHintScan(t *testing.T) {
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		h, _, _, err := computeModuleHints(m, 0, 0, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		// Preserve the length and type, but remove every loop opcode after scanning.
		body := append([]byte(nil), m.Code[0].BodyBytes...)
		for i := range body {
			body[i] = 1
		}
		m.Code[0].BodyBytes = body
		got := directMap32CodeReserve(m, 0, h[0], CompileOptions{AMD64Features: shared.AMD64ModernBaseline}, wasm.NewModuleInstructionClassifier(m, true))
		if got != map32CodeReserve {
			t.Fatalf("fp=%t: capacity re-scanned body: %d", fp, got)
		}
	}
}
