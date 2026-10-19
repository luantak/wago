//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
	"golang.org/x/sys/cpu"
	"testing"
)

func TestIntegerVectorCallRegion(t *testing.T) {
	requireCompilerDiagnostics(t)
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	old := integerMemoryAssertFast
	defer func() { integerMemoryAssertFast = old }()
	for _, paired := range []bool{false, true} {
		body := integerStoreModule(t, paired).Code[0].BodyBytes
		body = append(append([]byte{0, 0x10, 1}, body[:len(body)-1]...), 0x10, 1, 0x20, 5, 0xbd, 0xa7, 0x73, 0x0b)
		// The non-inlined helper has an observable write on each call, including
		// the initial call when a subsequent scalar memory access traps.
		helper := []byte{0, 0x41, 0xdc, 0xff, 3, 0x41, 0xdc, 0xff, 3, 0x28, 2, 0, 0x41, 1, 0x6a, 0x36, 2, 0, 0x0b}
		data := wasmtest.Module(
			wasmtest.Section(1, wasmtest.Vec(wasmtest.FuncType([]wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.F64}, []wasm.ValType{wasm.I32}), wasmtest.FuncType(nil, nil))),
			wasmtest.Section(3, wasmtest.Vec([]byte{0}, []byte{1})),
			wasmtest.Section(5, wasmtest.Vec([]byte{0, 1})),
			wasmtest.Section(7, wasmtest.Vec(wasmtest.ExportEntry("f", 0, 0))),
			wasmtest.Section(10, wasmtest.Vec(append(wasmtest.ULEB(uint32(len(body))), body...), append(wasmtest.ULEB(uint32(len(helper))), helper...))),
		)
		m, err := wasm.DecodeModule(data)
		if err != nil {
			t.Fatal(err)
		}
		init := func(mem []byte) {
			for i := range mem {
				mem[i] = byte(i*13 + 19)
			}
		}
		for _, n := range []uint64{0, 1, 7, 8, 9, 16, 17, 33} {
			for _, start := range []uint64{0, 0xfffffff0} {
				for _, bases := range [][2]uint64{{0, 512}, {1, 513}, {0, 0}, {0, 4}, {65520, 0}, {0, 65520}, {0xfffffffc, 0}} {
					// Preserve an unrelated floating-point argument, including its
					// NaN payload, across vector register use and the trailing call.
					args := []uint64{uint64(uint32(start + n)), start, bases[0], bases[1], 0xf1234567, 0x7ff8000012345678}
					integerMemoryAssertFast = false
					want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-store-vector": true, "integer-region-vector": false, "inline": false}}, init, args...)
					integerMemoryAssertFast = start == 0 && n >= 8 && bases[0] <= 1 && bases[1] >= 512 && bases[1] <= 513
					var stats ModuleStats
					got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline | shared.AMD64AVX2, Stats: &stats, Optimizations: map[string]bool{"integer-store-vector": true, "integer-region-vector": true, "inline": false}}, init, args...)
					integerMemoryAssertFast = false
					if (we != nil) != (ge != nil) || we == nil && got != want || !bytes.Equal(wm, gm) {
						t.Fatalf("paired=%v n=%d bases=%x got=%x/%v want=%x/%v memory=%v", paired, n, bases, got, ge, want, we, bytes.Equal(wm, gm))
					}
					if stats.Funcs[0].Peephole["integer-store-vector"] != 1 {
						t.Fatal("call-containing function did not vectorize call-free loop")
					}
				}
			}
		}
	}
}
