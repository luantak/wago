//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

func TestDirectMap32RecognitionAllocations(t *testing.T) {
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
			r.Bytes(at + 2)
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

func TestDirectMap32AscendingSteps(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, step := range []byte{1, 2, 4} {
			for _, n := range []uint64{16, 17, 65} {
				var result uint64
				var memory []byte
				for _, scalar := range []bool{true, false} {
					m := directMap32Fixture(t, fp, scalar)
					m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 2, 0x41, 1, 0x6a}, []byte{0x20, 2, 0x41, step, 0x6a}, 1)
					setup := func(mem []byte) {
						for i := 0; i < 65; i++ {
							binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)+0x3f800001)
						}
					}
					// A guard-failure trap proves that the candidate took the SIMD
					// path and entered the checked tail only after full groups.
					saved := regionLoopTestFast
					regionLoopTestFast = !scalar
					var stats ModuleStats
					got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: optionalTestStats(&stats)}, setup, 0, 32768, 7, 7+n*uint64(step), 3, 7)
					regionLoopTestFast = saved
					if !scalar && diagnosticsEnabled && stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
						t.Fatal("ascending candidate was not selected", fp, step, n, stats.Funcs[0].Peephole)
					}
					if err != nil {
						t.Fatal("ascending fast path", fp, step, n, err)
					}
					if scalar {
						result, memory = got, mem
					} else if got != result || !bytes.Equal(mem, memory) {
						t.Fatal("ascending locals or memory changed", fp, step, n)
					}
				}
			}
			if step == 1 {
				continue
			}
			// Planned ranges fit. Only the nondivisible count rejects this
			// otherwise qualified input; the diagnostic trap occurs before stores.
			m := directMap32Fixture(t, fp, false)
			m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 2, 0x41, 1, 0x6a}, []byte{0x20, 2, 0x41, step, 0x6a}, 1)
			var stats ModuleStats
			saved := regionLoopTestFast
			regionLoopTestFast = true
			_, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: optionalTestStats(&stats)}, nil, 0, 32768, 7, 7+16*uint64(step)+1, 3, 7)
			regionLoopTestFast = saved
			trap, ok := err.(*coreruntime.TrapError)
			if !ok || trap.Code != coreruntime.TrapUnreachable || !bytes.Equal(mem, make([]byte, len(mem))) {
				t.Fatal("divisibility guard did not reject before stores", fp, step, err)
			}
			if diagnosticsEnabled && stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
				t.Fatal("divisibility candidate was not selected")
			}
			// A nondivisible distance must retain modular scalar execution.
			// It reaches an OOB store long before the counter can wrap.
			var memory []byte
			var trapCode coreruntime.TrapCode
			for _, scalar := range []bool{true, false} {
				m := directMap32Fixture(t, fp, scalar)
				m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 2, 0x41, 1, 0x6a}, []byte{0x20, 2, 0x41, step, 0x6a}, 1)
				_, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}, nil, 65504, 0, 7, 7+16*uint64(step)+1, 3, 7)
				trap, ok := err.(*coreruntime.TrapError)
				if !ok {
					t.Fatal("nondivisible distance did not trap", err)
				}
				if scalar {
					memory, trapCode = mem, trap.Code
				} else if !bytes.Equal(memory, mem) || trapCode != trap.Code {
					t.Fatal("nondivisible fallback changed effects", fp, step)
				}
			}
		}
	}
}

func TestDirectMap32PressureAndPinnedLocals(t *testing.T) {
	for _, fp := range []bool{false, true} {
		var result uint64
		var memory []byte
		for _, scalar := range []bool{true, false} {
			base := directMap32Fixture(t, fp, scalar)
			ft, _ := base.LocalFuncType(0)
			params := append([]wasm.ValType(nil), ft.Params...)
			params = append(params, ft.Params[4])
			for i := 0; i < 8; i++ {
				params = append(params, wasm.I32)
			}
			body := append([]byte{1, 1, 0x7f}, base.Code[0].BodyBytes...)
			body = body[:len(body)-1]
			for i := byte(7); i < 15; i++ {
				body = append(body, 0x20, i, 0x73)
			}
			body = append(body, 0xb)
			m := modMem(t, 1, params, ft.Results, body)
			if err := wasm.ValidateModule(m); err != nil {
				t.Fatal(err)
			}
			setup := func(mem []byte) {
				for i := 0; i < 64; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i)+0x3f800000)
				}
			}
			args := []uint64{0, 32768, 0, 64, 3, 7, 0, 17, 19, 23, 29, 31, 37, 41, 43}
			got, mem, err := runMemAmd64(t, m, setup, args...)
			if err != nil {
				t.Fatal(err)
			}
			if scalar {
				result, memory = got, mem
			} else if got != result || !bytes.Equal(mem, memory) {
				t.Fatal("pressure changed local or memory state", fp, got, result)
			}
		}
	}
}

func TestDirectMap32StoresBeforeSuffixTrap(t *testing.T) {
	for _, fp := range []bool{false, true} {
		var originalPC uint32
		var originalMemory []byte
		for _, scalar := range []bool{true, false} {
			m := directMap32Fixture(t, fp, scalar)
			body := m.Code[0].BodyBytes
			body = append(body[:len(body)-1], 0x41, 0, 0x6e, 0xb)
			m.Code[0].BodyBytes = body
			setup := func(mem []byte) {
				for i := 0; i < 64; i++ {
					binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i))
				}
			}
			_, mem, err := runMemAmd64(t, m, setup, 0, 32768, 0, 64, 3, 7)
			trap, ok := err.(*coreruntime.TrapError)
			if !ok || trap.Code != coreruntime.TrapDivZero || len(trap.Frames) != 1 || !trap.Frames[0].HasProgramCounter {
				t.Fatal("suffix trap has no precise position", err)
			}
			if scalar {
				originalPC, originalMemory = trap.Frames[0].ProgramCounter, mem
			} else if trap.Frames[0].ProgramCounter+1 != originalPC || !bytes.Equal(mem, originalMemory) {
				t.Fatal("suffix trap position or prior stores changed", fp, trap, originalPC)
			}
		}
	}
}

func TestDirectMap32IndexedMemoryRejection(t *testing.T) {
	m := directMap32Fixture(t, false, false)
	m.Memories = append(m.Memories, m.Memories[0])
	m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x28, 0, 0}, []byte{0x28, 0x40, 1, 0}, 1)
	if err := wasm.ValidateModuleWithFeatures(m, wasm.ValidationFeatures{MultiMemory: true}); err != nil {
		t.Fatal(err)
	}
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Stats: optionalTestStats(&stats)})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if diagnosticsEnabled && stats.Funcs[0].Peephole["region-loop-map32"] != 0 {
		t.Fatal("indexed memory was vectorized")
	}
}

func TestDirectMap32LiveOperandPrefix(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, floatPrefix := range []bool{false, true} {
			var want uint64
			var expected []byte
			for _, scalar := range []bool{true, false} {
				m := directMap32Fixture(t, fp, scalar)
				body := m.Code[0].BodyBytes
				prefix := []byte{0x41, 37}
				suffix := []byte{0x6a, 0x0b}
				if floatPrefix {
					prefix = []byte{0x43, 0, 0, 0x40, 0x40}
					suffix = []byte{0xbe, 0x92, 0xbc, 0x0b}
				}
				m.Code[0].BodyBytes = append(append(prefix, body[:len(body)-1]...), suffix...)
				if err := wasm.ValidateModule(m); err != nil {
					t.Fatal(err)
				}
				saved := regionLoopTestFast
				regionLoopTestFast = !scalar
				var stats ModuleStats
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: optionalTestStats(&stats)}, func(mem []byte) {
					for i := 0; i < 65; i++ {
						binary.LittleEndian.PutUint32(mem[32768+4*i:], uint32(i))
					}
				}, 0, 32768, 0, 65, 3, 7)
				regionLoopTestFast = saved
				if !scalar && diagnosticsEnabled && stats.Funcs[0].Peephole["region-loop-map32"] != 1 {
					t.Fatal("live-prefix candidate was not selected", fp, floatPrefix, stats.Funcs[0].Peephole)
				}
				if err != nil {
					t.Fatal(err)
				}
				if scalar {
					want, expected = got, mem
				} else if want != got || !bytes.Equal(mem, expected) {
					t.Fatal("live operand changed", fp, floatPrefix, want, got)
				}
			}
		}
	}
}
