//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestIntegerMapVector(t *testing.T) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	body := []byte{0, 0x02, 0x40, 0x20, 0, 0x20, 1, 0x46, 0x0d, 0, 0x03, 0x40,
		0x20, 2, 0x20, 3, 0x28, 2, 12, 0x41, 7, 0x73, 0x36, 2, 4,
		0x20, 2, 0x41, 4, 0x6a, 0x21, 2,
		0x20, 3, 0x41, 4, 0x6a, 0x21, 3,
		0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b,
		0x0b, 0x20, 1, 0x20, 2, 0x73, 0x20, 3, 0x73, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*13 + 71)
		}
	}
	for _, wide := range []bool{false, true} {
		features := shared.AMD64ModernBaseline
		if wide {
			features |= shared.AMD64AVX2
		}
		for _, n := range []uint64{0, 1, 7, 8, 9, 17, 33} {
			for _, start := range []uint64{0, 0xfffffff0} {
				for _, ptr := range [][2]uint64{{0, 1024}, {1, 1025}, {1024, 0}, {0, 0}, {0, 4}, {4, 0}, {0, 65500}, {65500, 0}, {0xfffffffc, 0}, {0, 0xfffffffc}} {
					args := []uint64{uint64(uint32(start + n)), start, ptr[0], ptr[1]}
					integerMemoryAssertFast = false
					want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-map-vector": false}}, init, args...)
					integerMemoryAssertFast = wide && n >= 8 && start == 0 && ptr[0] <= 1 && ptr[1] >= 1024 && ptr[1] <= 1025
					var stats ModuleStats
					got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Stats: &stats, Optimizations: map[string]bool{"integer-map-vector": true}}, init, args...)
					if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
						t.Fatalf("wide=%v n=%d start=%x ptr=%x got=%x/%v want=%x/%v memory=%v", wide, n, start, ptr, got, ge, want, we, bytes.Equal(wm, gm))
					}
					if (stats.Funcs[0].Peephole["integer-map-vector"] == 1) != wide {
						t.Fatal("map emitter admission mismatch")
					}
				}
			}
		}
	}
}

func TestIntegerMapPairedStreams(t *testing.T) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	m := integerStoreModule(t, true)
	body := m.Code[0].BodyBytes
	body = bytes.Replace(body, []byte{0x20, 1, 0x41, 17, 0x6c, 0x20, 4, 0x73}, []byte{0x20, 3, 0x28, 2, 0, 0x20, 4, 0x73}, 1)
	body = bytes.Replace(body, []byte{0x20, 1, 0x41, 7, 0x6a}, []byte{0x20, 3, 0x28, 2, 4, 0x41, 7, 0x6a}, 1)
	m.Code[0].BodyBytes = body
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*31 + 5)
		}
	}
	for _, paired := range []bool{false, true} {
		for _, n := range []uint64{7, 8, 9, 17, 33} {
			for _, ptr := range [][2]uint64{{0, 1024}, {1, 1025}, {1024, 0}, {0, 0}, {0, 4}, {4, 0}, {0, 65504}, {65504, 0}} {
				args := []uint64{n, 0, ptr[0], ptr[1], 0xabcdef01}
				integerMemoryAssertFast = false
				want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-map-vector": false}}, init, args...)
				integerMemoryAssertFast = n >= 8 && ptr[0] <= 1 && ptr[1] >= 1024 && ptr[1] <= 1025
				var stats ModuleStats
				got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2, Stats: &stats, Optimizations: map[string]bool{"integer-map-vector": true, "integer-paired-loads": paired}}, init, args...)
				if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
					t.Fatalf("paired=%v n=%d ptr=%x got=%x/%v want=%x/%v memory=%v", paired, n, ptr, got, ge, want, we, bytes.Equal(wm, gm))
				}
				if stats.Funcs[0].Peephole["integer-map-vector"] != 1 {
					t.Fatal("map emitter not selected")
				}
				if (stats.Funcs[0].Peephole["integer-paired-loads"] > 0) != paired {
					t.Fatal("paired load selection mismatch")
				}
			}
		}
	}
}

func TestIntegerMapQuadStreams(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		testIntegerMapQuadStreams(t, grouped)
	}
}

func testIntegerMapQuadStreams(t *testing.T, grouped bool) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	for _, order := range [][4]byte{{0, 1, 2, 3}, {3, 1, 0, 2}} {
		body := []byte{0, 0x02, 0x40, 0x20, 0, 0x20, 1, 0x46, 0x0d, 0, 0x03, 0x40}
		for _, field := range order {
			body = append(body, 0x20, 2, 0x20, 3, 0x28, 2, field*4, 0x41, 7, 0x73, 0x36, 2, field*4)
		}
		body = append(body, 0x20, 2, 0x41, 16, 0x6a, 0x21, 2, 0x20, 3, 0x41, 16, 0x6a, 0x21, 3, 0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b, 0x0b, 0x20, 1, 0x20, 2, 0x73, 0x20, 3, 0x73, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		init := func(mem []byte) {
			for i := range mem {
				mem[i] = byte((i * 13) ^ (i >> 7))
			}
		}
		for _, n := range []uint64{0, 1, 7, 8, 9, 17, 33} {
			for _, ptr := range [][2]uint64{{0, 1024}, {1, 1025}, {1024, 0}, {0, 0}, {0, 4}, {4, 0}, {1148, 1024}, {1152, 1024}, {1024, 1148}, {1024, 1152}, {0, 65408}, {0, 65409}, {65520, 0}, {0xfffffffc, 0}, {0, 0xfffffffc}} {
				args := []uint64{n, 0, ptr[0], ptr[1]}
				integerMemoryAssertFast = false
				want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-map-vector": false}}, init, args...)
				integerMemoryAssertFast = n >= 8 && ptr[0] <= 1 && ptr[1] >= 1024 && ptr[1] <= 1025 || n == 8 && ptr[0] == 0 && ptr[1] == 65408
				integerMemoryAssertFast = integerMemoryAssertFast || n == 8 && (ptr == [2]uint64{1152, 1024} || ptr == [2]uint64{1024, 1152})
				var stats ModuleStats
				got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2, Stats: &stats, Optimizations: map[string]bool{"integer-map-vector": true, "integer-map-group-guard": grouped}}, init, args...)
				if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
					t.Fatalf("order=%v n=%d ptr=%x got=%x/%v want=%x/%v memory=%v", order, n, ptr, got, ge, want, we, bytes.Equal(wm, gm))
				}
				if stats.Funcs[0].Peephole["integer-map-quad-loads"] != 1 || stats.Funcs[0].Peephole["integer-map-quad-stores"] != 1 {
					t.Fatalf("quad emission missing: %v", stats.Funcs[0].Peephole)
				}
				guards := 16
				if grouped {
					guards = 1
				}
				if stats.Funcs[0].Peephole["integer-map-overlap-guard"] != guards {
					t.Fatalf("grouped=%v overlap guards=%d want=%d", grouped, stats.Funcs[0].Peephole["integer-map-overlap-guard"], guards)
				}
			}
		}
	}
}
