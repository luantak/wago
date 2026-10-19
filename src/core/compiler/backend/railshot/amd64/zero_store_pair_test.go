//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestZeroStorePair(t *testing.T) {
	for _, count := range []int{2, 4} {
		testZeroStorePair(t, false, count)
		testZeroStorePair(t, true, count)
	}
}

func testZeroStorePair(t *testing.T, loop bool, count int) {
	requireCompilerDiagnostics(t)
	for _, descending := range []bool{false, true} {
		body := []byte{0}
		for i := 0; i < count; i++ {
			off := i * 8
			if descending {
				off = (count - 1 - i) * 8
			}
			body = append(body, 0x20, 0, 0x42, 0, 0x37, 0, byte(off))
		}
		body = append(body, 0x41, 0, 0x0b)
		if loop {
			body = append(append([]byte{0, 0x03, 0x40}, body[1:len(body)-3]...), 0x0b, 0x41, 0, 0x0b)
		}
		m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, base := range []uint64{0, 1, 65504, 65505, 65520, 65521, 65528, 0xfffffff0, 0xffffffff} {
			init := func(mem []byte) {
				for i := range mem {
					mem[i] = 0x7b
				}
			}
			want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"zero-store-pair": false}}, init, base)
			var stats ModuleStats
			got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"zero-store-pair": true}}, init, base)
			if (we != nil) != (ge != nil) || we == nil && got != want || !bytes.Equal(wm, gm) {
				t.Fatalf("descending=%v base=%x got=%d/%v want=%d/%v", descending, base, got, ge, want, we)
			}
			if (stats.Funcs[0].Peephole["zero-store-pair"] > 0) != (descending && loop && count == 4) {
				t.Fatalf("admission descending=%v stats=%v", descending, stats.Funcs[0].Peephole)
			}
		}
	}
}
