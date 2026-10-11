//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"reflect"
	"testing"
	"unsafe"

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
			if directMap32CodeReserve(candidate, 0, h.funcHints, &opts) == 0 {
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
			opts := CompileOptions{AMD64Features: shared.AMD64ModernBaseline}
			reserve := directMap32CodeReserve(m, 0, h.funcHints, &opts)
			want := map32CodeReserve
			if rejected {
				want = 0
			}
			if reserve != want {
				t.Fatalf("fp=%t rejected=%t reserve=%d want=%d", fp, rejected, reserve, want)
			}
			if allocs := testing.AllocsPerRun(1000, func() { directMap32CodeReserve(m, 0, h.funcHints, &opts) }); allocs != 0 {
				t.Fatalf("reserve allocates %g objects", allocs)
			}
			opts.AMD64Features = 0
			if !fp && directMap32CodeReserve(m, 0, h.funcHints, &opts) != 0 {
				t.Fatal("unsupported i32 CPU reserves space")
			}
			opts.AMD64Features = shared.AMD64ModernBaseline
			opts.Interruptible = true
			if directMap32CodeReserve(m, 0, h.funcHints, &opts) != 0 {
				t.Fatal("interruptible function reserves space")
			}
			opts.Interruptible = false
			m.Memories[0].Shared = true
			if directMap32CodeReserve(m, 0, h.funcHints, &opts) != 0 {
				t.Fatal("shared memory reserves space")
			}
			m.Memories[0].Shared = false
			m.Memories[0].Limits.Addr64 = true
			if directMap32CodeReserve(m, 0, h.funcHints, &opts) != 0 {
				t.Fatal("memory64 reserves space")
			}
			m.Memories[0].Limits.Addr64 = false
			m.Code[0].BodyBytes = append(m.Code[0].BodyBytes, make([]byte, regionLoopMaxBytes)...)
			if directMap32CodeReserve(m, 0, h.funcHints, &opts) != 0 {
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
		got := directMap32CodeReserve(m, 0, h.funcHints, &CompileOptions{AMD64Features: shared.AMD64ModernBaseline})
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
		got := directMap32CodeReserve(m, 0, h[0], &CompileOptions{AMD64Features: shared.AMD64ModernBaseline})
		if got != map32CodeReserve {
			t.Fatalf("fp=%t: capacity re-scanned body: %d", fp, got)
		}
	}
}

func TestDirectMap32HintPacking(t *testing.T) {
	if unsafe.Sizeof(funcHints{}) != 28 {
		t.Fatal("retained hints grew")
	}
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		fn := m.Code[0]
		for len(m.Code) < 512 {
			m.Code = append(m.Code, fn)
			m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
		}
		if !parallelHintScanEligible(m, 0, 4) {
			t.Fatal("parallel scan fixture too small")
		}
		for _, workers := range []int{1, 4} {
			hints, _, _, err := computeModuleHintsWithWorkersPolicy(m, 0, 0, workers, nil, false, currentCodegenPolicy())
			if err != nil {
				t.Fatal(err)
			}
			for _, h := range hints {
				if h.map32HintCount() != 1 {
					t.Fatal("map count lost", fp, workers, h.map32HintCount())
				}
				density := h.immediateFreeOpCount()
				h.noteFixedScratchLease()
				if !h.hasFixedScratchLease() || h.immediateFreeOpCount() != density || h.map32HintCount() != 1 {
					t.Fatal("packed hints interfere")
				}
			}
		}
	}
}

func TestDirectMap32MixedPackingAndNestedShape(t *testing.T) {
	s := byteBodyScanner{}
	s.h.noteFixedScratchLease()
	for i := 0; i < 300; i++ {
		s.noteImmediateFreeOp()
	}
	for i := 0; i < 5; i++ {
		s.map32Shape = map32HintActive | map32HintBranch | map32HintLoadI32 | map32HintStoreI32 | map32HintMulI32
		if i%2 != 0 {
			s.map32Shape = map32HintActive | map32HintBranch | map32HintLoadF32 | map32HintStoreF32 | map32HintMulF32
		}
		s.finishMap32Hint()
	}
	s.noteImmediateFreeOp()
	s.h.noteFixedScratchLease()
	if s.h.immediateFreeOpCount() != 256 || !s.h.hasFixedScratchLease() || s.h.map32HintCount() != 4 {
		t.Fatal("packed field interference", s.h)
	}
	m := directMap32Fixture(t, false, false)
	if got := directMap32CodeReserve(m, 0, s.h.funcHints, &CompileOptions{}); got != 2*map32CodeReserve {
		t.Fatal("CPU filter lost mixed counts", got)
	}
	original := m.Code[0].BodyBytes
	start := bytes.Index(original, []byte{3, 0x40})
	stop := start + bytes.Index(original[start:], []byte{0x0d, 0, 0x0b}) + 3
	loop := original[start:stop]
	body := append([]byte{3, 0x40}, loop...)
	body = append(body, 0x0b)
	body = append(body, loop...)
	body = append(body, 0x41, 0, 0x0b)
	m.Code[0].BodyBytes = body
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	h, _, _, err := computeModuleHints(m, 0, 0, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if h[0].map32HintCount() != 2 {
		t.Fatal("nested/sibling map counts", h[0].map32HintCount())
	}
}

func TestDirectMap32ConservativeReserveKeepsScalar(t *testing.T) {
	m := directMap32Fixture(t, false, false)
	m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 4, 0x6c}, []byte{0x20, 4, 0x6c, 0x41, 0, 0x73}, 1)
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	h, _, _, err := computeModuleHints(m, 0, 0, nil, false)
	if err != nil || h[0].map32HintCount() != 1 {
		t.Fatal("false-positive fixture", err)
	}
	if diagnosticsEnabled {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: &stats})
		if err != nil {
			t.Fatal(err)
		}
		cm.CodeImage.Close()
		if stats.Funcs[0].Peephole["region-loop-map32"] != 0 {
			t.Fatal("sizing hint authorized SIMD")
		}
	}
	args := []uint64{128, 1024, 0, 32, 2, 1}
	setup := func(mem []byte) {
		for i := 0; i < 32; i++ {
			binary.LittleEndian.PutUint32(mem[1024+4*i:], uint32(i))
		}
	}
	a, am, ae := runMemAmd64WithOptions(t, directMap32Fixture(t, false, true), CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}, setup, args...)
	b, bm, be := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}, setup, args...)
	if ae != nil || be != nil || !reflect.DeepEqual(a, b) || !bytes.Equal(am, bm) {
		t.Fatal("false-positive reserve changed scalar result", ae, be)
	}
}
