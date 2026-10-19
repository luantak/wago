//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestByteBoundsDirect(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, store := range []bool{false, true} {
		for _, offset := range []byte{0, 1} {
			body := []byte{0, 0x20, 0}
			if store {
				body = append(body, 0x41, 42, 0x3a, 0, offset, 0x41, 7)
			} else {
				body = append(body, 0x2d, 0, offset)
			}
			body = append(body, 0x0b)
			for _, pages := range []uint32{0, 1} {
				m := modMem(t, pages, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
				for _, addr := range []uint64{0, 1, 65534, 65535, 65536, 0xffffffff, 0x100000000} {
					init := func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*13 + 7)
						}
					}
					want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"byte-bounds-direct": false}}, init, addr)
					var stats ModuleStats
					got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"byte-bounds-direct": true}}, init, addr)
					if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
						t.Fatalf("store=%v offset=%d pages=%d addr=%x got=%x/%v want=%x/%v", store, offset, pages, addr, got, ge, want, we)
					}
					if (stats.Funcs[0].Peephole["byte-bounds-direct"] == 1) != (offset == 0) {
						t.Fatalf("bounds selection: %v", stats.Funcs[0].Peephole)
					}
				}
			}
		}
	}
}
