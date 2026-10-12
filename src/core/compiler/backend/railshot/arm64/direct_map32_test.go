//go:build (linux || darwin || windows) && arm64

package arm64

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

func directMap32Fixture(t testing.TB, fp, scalar bool) *wasm.Module {
	t.Helper()
	typ, load, mul, add, store := wasm.I32, byte(0x28), byte(0x6c), byte(0x6a), byte(0x36)
	if fp {
		typ, load, mul, add, store = wasm.F32, 0x2a, 0x94, 0x92, 0x38
	}
	body := []byte{1, 1, map[bool]byte{false: 0x7f, true: 0x7d}[fp], 0x02, 0x40, 0x20, 3, 0x45, 0x0d, 0, 0x03, 0x40}
	if scalar {
		body = append(body, 0x01)
	}
	body = append(body, 0x20, 0, 0x20, 1, load, 0, 0, 0x20, 4, mul, 0x20, 5, add, 0x22, 6, store, 0, 0,
		0x20, 0, 0x41, 4, 0x6a, 0x21, 0, 0x20, 1, 0x41, 4, 0x6a, 0x21, 1,
		0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x20, 3, 0x47, 0x0d, 0, 0x0b, 0x0b,
		0x20, 0, 0x20, 1, 0x73, 0x20, 2, 0x73, 0x20, 6)
	if fp {
		body = append(body, 0xbc)
	}
	body = append(body, 0x73, 0x0b)
	return modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, typ, typ}, []wasm.ValType{wasm.I32}, body)
}

func runMemArm64WithOptions(t *testing.T, m *wasm.Module, opts CompileOptions, setup func([]byte), args ...uint64) (uint64, []byte, error) {
	t.Helper()
	cm, err := CompileModuleWith(m, opts)
	if err != nil {
		t.Fatalf("arm64 compile: %v", err)
	}
	if cm.CodeImage != nil {
		defer cm.CodeImage.Close()
	}
	eng, err := coreruntime.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	jm, err := coreruntime.NewJobMemory(1 << 16)
	if err != nil {
		t.Fatal(err)
	}
	defer jm.Close()
	ar, err := coreruntime.NewArena(4096)
	if err != nil {
		t.Fatal(err)
	}
	defer ar.Close()
	lin := jm.LinearMemory()
	if setup != nil {
		setup(lin)
	}
	code, entry, err := coreruntime.MapCode(cm.Code)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	defer coreruntime.Unmap(code)
	serArgs, results, trap := ar.Alloc(256), ar.Alloc(256), ar.Alloc(coreruntime.TrapBufferBytes)
	for i, arg := range args {
		binary.LittleEndian.PutUint64(serArgs[i*8:], arg)
	}
	callErr := eng.Call(entry+uintptr(cm.Entry[0]), serArgs, lin, trap, results)
	return binary.LittleEndian.Uint64(results), append([]byte(nil), lin...), callErr
}

func TestDirectMap32SelectionARM64(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, fp := range []bool{false, true} {
		for _, scalar := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(directMap32Fixture(t, fp, scalar), CompileOptions{Stats: &stats})
			if err != nil {
				t.Fatal(err)
			}
			if cm.CodeImage != nil {
				cm.CodeImage.Close()
			}
			want := !scalar
			if got := stats.Funcs[0].Peephole["region-loop-map32"] != 0; got != want {
				t.Fatalf("fp=%v scalar=%v SIMD=%v want=%v stats=%v", fp, scalar, got, want, stats.Funcs[0].Peephole)
			}
		}
	}
}

func TestDirectMap32SemanticsARM64(t *testing.T) {
	for _, fp := range []bool{false, true} {
		ref, candidate := directMap32Fixture(t, fp, true), directMap32Fixture(t, fp, false)
		for _, n := range []uint64{0, 1, 15, 16, 17, 20, 24, 28, 31, 32, 33, 36, 40, 44, 48, 52, 56, 60, 63, 64, 65, 127, 128, 129, 513, 8192} {
			for _, pair := range [][2]uint64{{128, 32768}, {129, 32769}, {128, 128}, {129, 128}, {128, 129}, {128, 192}, {192, 128}, {128, 65520}, {65520, 128}} {
				setup := func(mem []byte) {
					for i := range mem {
						mem[i] = 0xa5
					}
					values := [...]uint32{0, 0x80000000, 1, 0x7f800000, 0xff800000, 0x7fc12345, 0x3f800001, 0x3f7fffff, 0xffffffff, 0x80000001}
					for i := uint64(0); i < n && pair[1]+4*i+4 <= 65536; i++ {
						binary.LittleEndian.PutUint32(mem[pair[1]+4*i:], values[i%uint64(len(values))])
					}
				}
				scale, bias := uint64(0xffffffff), uint64(0x80000001)
				if fp {
					scale, bias = uint64(math.Float32bits(1.0000001)), uint64(math.Float32bits(-1))
				}
				a, am, ae := runMemArm64WithOptions(t, ref, CompileOptions{}, setup, pair[0], pair[1], 0, n, scale, bias)
				b, bm, be := runMemArm64WithOptions(t, candidate, CompileOptions{}, setup, pair[0], pair[1], 0, n, scale, bias)
				if (ae == nil) != (be == nil) || ae == nil && a != b || !bytes.Equal(am, bm) {
					t.Fatalf("fp=%v n=%d pair=%v results=%x/%x traps=%v/%v", fp, n, pair, a, b, ae, be)
				}
				if ae != nil {
					x, xok := ae.(*coreruntime.TrapError)
					y, yok := be.(*coreruntime.TrapError)
					if !xok || !yok || x.Code != y.Code {
						t.Fatalf("fp=%v n=%d pair=%v traps=%v/%v", fp, n, pair, ae, be)
					}
				}
			}
		}
	}
}

func TestDirectMap32RejectsNonCountdownHeaderExitARM64(t *testing.T) {
	for _, fp := range []bool{false, true} {
		var want uint64
		var expected []byte
		for _, scalar := range []bool{true, false} {
			m := directMap32Fixture(t, fp, scalar)
			body := m.Code[0].BodyBytes
			at := bytes.Index(body, []byte{0x03, 0x40})
			if at < 0 {
				t.Fatal("loop marker missing")
			}
			header := []byte{0x20, 2, 0x45, 0x0d, 1}
			body = append(append(append([]byte(nil), body[:at+2]...), header...), body[at+2:]...)
			m.Code[0].BodyBytes = body
			if err := wasm.ValidateModule(m); err != nil {
				t.Fatal(err)
			}
			var stats ModuleStats
			opts := CompileOptions{}
			if diagnosticsEnabled {
				opts.Stats = &stats
			}
			got, mem, err := runMemArm64WithOptions(t, m, opts, func(mem []byte) {
				for i := 0; i < 64; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)+1)
				}
			}, 0, 32768, 0, 64, 3, 7)
			if err != nil {
				t.Fatal(err)
			}
			if !scalar && diagnosticsEnabled && stats.Funcs[0].Peephole["region-loop-map32"] != 0 {
				t.Fatal("non-countdown header exit was vectorized")
			}
			if scalar {
				want, expected = got, mem
			} else if got != want || !bytes.Equal(mem, expected) {
				t.Fatal("ignored header exit changed locals or memory", fp)
			}
		}
	}
}

func directMap32Countdown(t testing.TB, fp, scalar bool) *wasm.Module {
	t.Helper()
	m := directMap32Fixture(t, fp, scalar)
	body := m.Code[0].BodyBytes
	outer := []byte{0x02, 0x40, 0x20, 3, 0x45, 0x0d, 0, 0x03, 0x40}
	header := []byte{0x02, 0x40, 0x03, 0x40, 0x20, 3, 0x45, 0x0d, 1}
	body = bytes.Replace(body, outer, header, 1)
	latch := []byte{0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x20, 3, 0x47, 0x0d, 0}
	body = bytes.Replace(body, latch, []byte{0x20, 3, 0x41, 1, 0x6b, 0x21, 3, 0x0c, 0}, 1)
	m.Code[0].BodyBytes = body
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDirectMap32CountdownARM64(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, n := range []uint64{0, 1, 15, 16, 17, 20, 24, 28, 32, 33, 36, 40, 44, 48, 52, 56, 60, 63, 64, 65, 8192} {
			setup := func(mem []byte) {
				for i := uint64(0); i < n; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)|0x3f800000)
				}
			}
			scale, bias := uint64(3), uint64(7)
			if fp {
				scale, bias = uint64(math.Float32bits(1.0000001)), uint64(math.Float32bits(-1))
			}
			a, am, ae := runMemArm64WithOptions(t, directMap32Countdown(t, fp, true), CompileOptions{}, setup, 0, 32768, 0, n, scale, bias)
			b, bm, be := runMemArm64WithOptions(t, directMap32Countdown(t, fp, false), CompileOptions{}, setup, 0, 32768, 0, n, scale, bias)
			if ae != nil || be != nil || a != b || !bytes.Equal(am, bm) {
				t.Fatalf("fp=%v n=%d results=%x/%x traps=%v/%v", fp, n, a, b, ae, be)
			}
		}
	}
}

func TestDirectMap32AscendingStepsARM64(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, step := range []byte{1, 2, 4} {
			for _, n := range []uint64{16, 17, 65} {
				setup := func(mem []byte) {
					for i := 0; i < 65; i++ {
						binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)+0x3f800001)
					}
				}
				var result uint64
				var memory []byte
				for _, scalar := range []bool{true, false} {
					m := directMap32Fixture(t, fp, scalar)
					m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 2, 0x41, 1, 0x6a}, []byte{0x20, 2, 0x41, step, 0x6a}, 1)
					got, mem, err := runMemArm64WithOptions(t, m, CompileOptions{}, setup, 0, 32768, 7, 7+n*uint64(step), 3, 7)
					if err != nil {
						t.Fatalf("fp=%v step=%d n=%d: %v", fp, step, n, err)
					}
					if scalar {
						result, memory = got, mem
					} else if got != result || !bytes.Equal(mem, memory) {
						t.Fatalf("fp=%v step=%d n=%d changed result or memory", fp, step, n)
					}
				}
			}
		}
	}
}

func TestDirectMap32RecognitionAllocationsARM64(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, scalar := range []bool{false, true} {
			m := directMap32Fixture(t, fp, scalar)
			ft, _ := m.LocalFuncType(0)
			var types []machineType
			for _, v := range ft.Params {
				types = append(types, mtOf(v))
			}
			types = append(types, mtOf(ft.Params[4]))
			classifier := wasm.NewModuleInstructionClassifier(m, false)
			at := bytes.Index(m.Code[0].BodyBytes, []byte{3, 0x40})
			r := wasm.ReaderFrom(m.Code[0].BodyBytes)
			_, _ = r.Bytes(at + 2)
			var p regionLoopPlan
			allocs := testing.AllocsPerRun(1000, func() {
				accepted := inspectRegionLoop(r, types, classifier, &p) && p.qualifyMap32()
				if accepted == scalar {
					panic("unexpected map recognition")
				}
			})
			if allocs != 0 {
				t.Fatal("recognition allocated", fp, scalar, allocs)
			}
		}
	}
}

func TestDirectMap32CapacityHintARM64(t *testing.T) {
	packed := funcHints{immediateFreeOps: streamingReductionHintMask | 17 | 2<<map32I32HintShift | 1<<map32F32HintShift}
	if got := packed.immediateFreeOpCount(); got != 17 {
		t.Fatalf("packed density=%d, want 17", got)
	}
	if got := packed.map32HintCount(); got != 3 {
		t.Fatalf("packed map hints=%d, want 3", got)
	}
	if !packed.hasStreamingReduction() {
		t.Fatal("packed streaming-reduction hint was lost")
	}

	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		ft, _ := m.LocalFuncType(0)
		hints, err := scanFuncBody(m.Code[0], len(ft.Params)+1, 0, 0, nil, m)
		if err != nil {
			t.Fatal(err)
		}
		if got := hints.map32HintCount(); got != 1 {
			t.Fatalf("fp=%v map hints=%d, want 1", fp, got)
		}
		if got := directMap32CodeReserve(m, 0, hints.funcHints, &CompileOptions{}); got != map32CodeReserve {
			t.Fatalf("fp=%v reserve=%d, want %d", fp, got, map32CodeReserve)
		}
	}

	m := directMap32Fixture(t, false, false)
	body := m.Code[0].BodyBytes
	body = append(append(append([]byte(nil), body[:len(body)-1]...), bytes.Repeat([]byte{0x01}, regionLoopMaxBytes)...), 0x0b)
	m.Code[0].BodyBytes = body
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	ft, _ := m.LocalFuncType(0)
	hints, err := scanFuncBody(m.Code[0], len(ft.Params)+1, 0, 0, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	if got := hints.map32HintCount(); got != 0 {
		t.Fatalf("oversize body map hints=%d, want 0", got)
	}
}

func TestDirectMap32HeapAllocationBudgetARM64(t *testing.T) {
	if profileEnabled || regallocCheckEnabled {
		t.Skip("production allocation budget excludes profile and register-transfer instrumentation")
	}
	for _, fp := range []bool{false, true} {
		for _, countdown := range []bool{false, true} {
			count := func(scalar bool) float64 {
				m := directMap32Fixture(t, fp, scalar)
				if countdown {
					m = directMap32Countdown(t, fp, scalar)
				}
				return testing.AllocsPerRun(100, func() {
					cm, err := CompileModuleWith(m, CompileOptions{Workers: 1, DeferCodeMapping: true})
					if err != nil {
						t.Fatal(err)
					}
					if cm.CodeImage != nil {
						cm.CodeImage.Close()
					}
				})
			}
			scalar, candidate := count(true), count(false)
			if candidate > scalar {
				t.Fatalf("fp=%v countdown=%v scalar=%v SIMD=%v allocations", fp, countdown, scalar, candidate)
			}
		}
	}
}

func TestDirectMap32LiveFloatParametersARM64(t *testing.T) {
	// Keep many float parameters live across the loop. Additional vector
	// temporaries must never overwrite their homes or prevent scalar fallback.
	for _, extra := range []int{24, 25} {
		var result uint64
		var memory []byte
		for _, scalar := range []bool{true, false} {
			m := directMap32Fixture(t, true, scalar)
			ft, _ := m.LocalFuncType(0)
			for i := 0; i < extra; i++ {
				ft.Params = append(ft.Params, wasm.F32)
			}
			body := m.Code[0].BodyBytes
			body = bytes.ReplaceAll(body, []byte{0x22, 6}, []byte{0x22, byte(len(ft.Params))})
			body = bytes.ReplaceAll(body, []byte{0x20, 6}, []byte{0x20, byte(len(ft.Params))})
			body = body[:len(body)-1]
			for i := 6; i < len(ft.Params); i++ {
				body = append(body, 0x20, byte(i), 0xbc, 0x73)
			}
			m.Code[0].BodyBytes = append(body, 0x0b)
			if err := wasm.ValidateModule(m); err != nil {
				t.Fatal(err)
			}
			args := []uint64{0, 32768, 0, 65, uint64(math.Float32bits(1.0000001)), uint64(math.Float32bits(-1))}
			for i := 0; i < extra; i++ {
				args = append(args, uint64(math.Float32bits(float32(i)+0.25)))
			}
			setup := func(mem []byte) {
				for i := 0; i < 65; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)+0x3f800001)
				}
			}
			got, mem, err := runMemArm64WithOptions(t, m, CompileOptions{}, setup, args...)
			if err != nil {
				t.Fatal(err)
			}
			if scalar {
				result, memory = got, mem
			} else if got != result || !bytes.Equal(mem, memory) {
				t.Fatalf("extra=%d changed live parameters or memory", extra)
			}
		}
	}
}
