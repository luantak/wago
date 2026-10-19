//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"golang.org/x/sys/cpu"
	"testing"
)

func integerStoreModule(t *testing.T, paired bool) *wasm.Module {
	step, second, offset := byte(4), byte(3), byte(0)
	if paired {
		step, second, offset = 8, 2, 4
	}
	body := []byte{0, 0x02, 0x40, 0x20, 0, 0x20, 1, 0x46, 0x0d, 0, 0x03, 0x40,
		0x20, 2, 0x20, 1, 0x41, 17, 0x6c, 0x20, 4, 0x73, 0x36, 2, 0,
		0x20, second, 0x20, 1, 0x41, 7, 0x6a, 0x36, 2, offset,
		0x20, 2, 0x41, step, 0x6a, 0x21, 2,
		0x20, 3, 0x41, step, 0x6a, 0x21, 3,
		0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b,
		0x0b, 0x20, 1, 0x20, 2, 0x73, 0x20, 3, 0x73, 0x0b}
	return modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
}

func TestIntegerStoreVector(t *testing.T) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*13 + 19)
		}
	}
	for mode := 0; mode < 3; mode++ {
		paired := mode != 0
		m := integerStoreModule(t, paired)
		if mode == 2 {
			body := m.Code[0].BodyBytes
			a := bytes.Index(body, []byte{0x36, 2, 0})
			b := bytes.Index(body, []byte{0x36, 2, 4})
			body[a+2], body[b+2] = 4, 0
		}
		for _, wide := range []bool{false, true} {
			features := shared.AMD64ModernBaseline
			if wide {
				features |= shared.AMD64AVX2
			}
			for _, n := range []uint64{0, 1, 7, 8, 9, 16, 17, 33} {
				for _, start := range []uint64{0, 0xfffffff0} {
					limit := uint64(uint32(start + n))
					for _, bases := range [][2]uint64{{0, 512}, {1, 513}, {0, 0}, {0, 4}, {65520, 0}, {0, 65520}, {0xfffffffc, 0}} {
						args := []uint64{limit, start, bases[0], bases[1], 0xf1234567}
						integerMemoryAssertFast = false
						want, wantMem, wantErr := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-store-vector": false}}, init, args...)
						integerMemoryAssertFast = wide && n >= 8 && start == 0 && bases[0] <= 1 && bases[1] >= 512 && bases[1] <= 513
						var stats ModuleStats
						got, gotMem, gotErr := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Stats: &stats, Optimizations: map[string]bool{"integer-store-vector": true}}, init, args...)
						if (wantErr != nil) != (gotErr != nil) || wantErr == nil && got != want || !bytes.Equal(gotMem, wantMem) {
							t.Fatalf("paired=%v wide=%v n=%d start=%x bases=%x got=%x/%v want=%x/%v memoryEqual=%v", paired, wide, n, start, bases, got, gotErr, want, wantErr, bytes.Equal(gotMem, wantMem))
						}
						expected := 0
						if wide {
							expected = 1
						}
						if stats.Funcs[0].Peephole["integer-store-vector"] != expected {
							t.Fatal("store emitter selection mismatch")
						}
					}
				}
			}
		}
	}
}

func TestIntegerStoreCrossPairOverlap(t *testing.T) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	m := integerStoreModule(t, true)
	body := m.Code[0].BodyBytes
	at := bytes.Index(body, []byte{0x20, 2, 0x41, 8, 0x6a, 0x21, 2})
	extra := []byte{0x20, 3, 0x41, 23, 0x36, 2, 0, 0x20, 3, 0x41, 29, 0x36, 2, 4}
	m.Code[0].BodyBytes = append(append(append([]byte{}, body[:at]...), extra...), body[at:]...)
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*7 + 3)
		}
	}
	for _, base := range []uint64{0, 4, 8, 32, 128, 1024, 65520, 0xfffffffc} {
		args := []uint64{17, 0, 0, base, 0x87654321}
		integerMemoryAssertFast = false
		want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-store-vector": false}}, init, args...)
		integerMemoryAssertFast = base == 1024
		var stats ModuleStats
		got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2, Stats: &stats, Optimizations: map[string]bool{"integer-store-vector": true}}, init, args...)
		if (we != nil) != (ge != nil) || we == nil && got != want || !bytes.Equal(wm, gm) {
			t.Fatalf("cross-pair base=%x got=%x/%v want=%x/%v memoryEqual=%v", base, got, ge, want, we, bytes.Equal(wm, gm))
		}
		if stats.Funcs[0].Peephole["integer-store-vector"] != 1 {
			t.Fatal("cross-pair fixture did not select vector stores")
		}
	}
}
