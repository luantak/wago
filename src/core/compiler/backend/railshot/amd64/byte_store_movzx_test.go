//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestByteStoreMovzx(t *testing.T) {
	for _, descending := range []bool{false, true} {
		body := []byte{0}
		for i := 0; i < 4; i++ {
			off := i
			if descending {
				off = 3 - i
			}
			body = append(body, 0x20, 0, 0x20, byte(i+1), 0x3a, 0, byte(off))
		}
		body = append(body, 0x20, 0, 0x28, 0, 0, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, base := range []uint64{0, 1, 65532, 65533, 65534, 65535, 65536, 0xfffffffe, 0xffffffff} {
			args := []uint64{base, 0xdeadbeef, 0xffffffff, 0x12345678, 0xabcdef00}
			var stats ModuleStats
			want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"byte-store-run": false}}, nil, args...)
			got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"byte-store-run": true, "byte-store-movzx": true}}, nil, args...)
			if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
				t.Fatalf("descending=%v base=%x got=%x/%v want=%x/%v", descending, base, got, ge, want, we)
			}
			if (stats.Funcs[0].Peephole["byte-store-run"] > 0) != descending {
				t.Fatalf("missing admission descending=%v", descending)
			}
		}
	}
}

func TestByteStoreMovzxAliasingAndOffset(t *testing.T) {
	// The address may also supply stored bytes; repeated value locals must
	// survive destructive packing. A nonzero common displacement is allowed.
	body := []byte{0, 0x20, 0, 0x20, 0, 0x3a, 0, 7,
		0x20, 0, 0x20, 1, 0x3a, 0, 6,
		0x20, 0, 0x20, 1, 0x3a, 0, 5,
		0x20, 0, 0x20, 0, 0x3a, 0, 4,
		0x20, 1, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, base := range []uint64{0, 1, 255, 65528, 65529, 0xfffffffc} {
		init := func(mem []byte) {
			for i := range mem {
				mem[i] = 0x91
			}
		}
		want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"byte-store-run": false}}, init, base, 0xabcdef80)
		var stats ModuleStats
		got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"byte-store-run": true, "byte-store-movzx": true}}, init, base, 0xabcdef80)
		if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) || stats.Funcs[0].Peephole["byte-store-run"] != 1 {
			t.Fatalf("base=%x got=%x/%v want=%x/%v", base, got, ge, want, we)
		}
	}
}
