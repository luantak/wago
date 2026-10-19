//go:build linux && amd64

package amd64

import (
	"encoding/binary"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// Arithmetic i32 offsets wrap before the memory access; memarg offsets do not.
// A successful check for (base+32) therefore cannot by itself certify base+28.
func TestWrappedDescendingLoadBounds(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, barrier := range []bool{false, true} {
		body := []byte{0, 0x20, 0, 0x41, 32, 0x6a, 0x28, 2, 0, 0x1a}
		if barrier {
			// An observable store must happen after the first successful load
			// and before a failure at the second load.
			body = append(body, 0x41, 8, 0x41, 42, 0x36, 2, 0)
		}
		body = append(body, 0x20, 0, 0x41, 28, 0x6a, 0x28, 2, 0, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
		m.Memories[0].Limits.HasMax = true
		m.Memories[0].Limits.Max = 1
		for _, tc := range []struct {
			base  uint32
			trap  bool
			want  uint64
			store bool
		}{
			{0, false, 17, true},
			{0xffffffe0, true, 0, true},   // +32 wraps to 0; +28 stays at UINT32_MAX-3.
			{0xffffffe4, false, 11, true}, // Both offsets wrap, and both loads are valid.
			{65504, true, 0, false},       // First load fails, before the optional store.
		} {
			var stats ModuleStats
			got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"wrapped-load-group": true}}, func(mem []byte) {
				binary.LittleEndian.PutUint32(mem, 11)
				binary.LittleEndian.PutUint32(mem[28:], 17)
			}, uint64(tc.base))
			if (stats.Funcs[0].Peephole["wrapped-load-group"] > 0) == barrier {
				t.Fatalf("barrier=%v group admission: %v", barrier, stats.Funcs[0].Peephole)
			}
			if (err != nil) != tc.trap || !tc.trap && got != tc.want {
				t.Fatalf("barrier=%v base=%x got=%d err=%v want=%d trap=%v", barrier, tc.base, got, err, tc.want, tc.trap)
			}
			wantStore := uint32(0)
			if barrier && tc.store {
				wantStore = 42
			}
			if gotStore := binary.LittleEndian.Uint32(mem[8:]); gotStore != wantStore {
				t.Fatalf("barrier=%v base=%x store=%d want=%d", barrier, tc.base, gotStore, wantStore)
			}
		}
	}
}

func TestWrappedLoadGroupInvalidation(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{0, 0x20, 0, 0x41, 32, 0x6a, 0x28, 2, 0, 0x1a, 0x20, 0, 0x41, 28, 0x6a, 0x28, 2, 0, 0x1a,
		0x20, 1, 0x21, 0, 0x20, 0, 0x41, 28, 0x6a, 0x28, 2, 0, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, max := range []uint64{1, 65536} {
		m.Memories[0].Limits.HasMax = true
		m.Memories[0].Limits.Max = max
		var stats ModuleStats
		_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"wrapped-load-group": true}}, nil, 0, 65536)
		if err == nil {
			t.Fatalf("max=%d overwritten source did not trap", max)
		}
		if (stats.Funcs[0].Peephole["wrapped-load-group"] > 0) != (max == 1) {
			t.Fatalf("max=%d admission: %v", max, stats.Funcs[0].Peephole)
		}
	}
}
