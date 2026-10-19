//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSignedLoadGroup(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, reverse := range []bool{false, true} {
		for _, barrier := range []bool{false, true} {
			for _, max := range []uint64{1, 65536} {
				first, second := int64(-4), int64(0)
				if reverse {
					first, second = second, first
				}
				body := []byte{0, 0x20, 0, 0x41}
				body = append(body, sleb128(first)...)
				body = append(body, 0x6a, 0x28, 2, 0, 0x1a)
				if barrier {
					body = append(body, 0x41, 8, 0x41, 42, 0x36, 2, 0)
				}
				body = append(body, 0x20, 0, 0x41)
				body = append(body, sleb128(second)...)
				body = append(body, 0x6a, 0x28, 2, 0, 0x0b)
				m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
				m.Memories[0].Limits.HasMax = true
				m.Memories[0].Limits.Max = max
				for _, base := range []uint64{0, 1, 3, 4, 8, 65532, 65533, 65536, 0xfffffffc, 0xffffffff} {
					init := func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*31 + 7)
						}
					}
					var stats ModuleStats
					want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"wrapped-load-group": true, "signed-load-group": false}}, init, base)
					got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"wrapped-load-group": true, "signed-load-group": true}}, init, base)
					if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
						t.Fatalf("reverse=%v barrier=%v max=%d base=%x got=%d/%v want=%d/%v", reverse, barrier, max, base, got, ge, want, we)
					}
					if (stats.Funcs[0].Peephole["wrapped-load-group"] > 0) != (!reverse && !barrier && max == 1) {
						t.Fatalf("admission reverse=%v barrier=%v max=%d stats=%v", reverse, barrier, max, stats.Funcs[0].Peephole)
					}
				}
			}
		}
	}
}

func TestSignedLoadGroupRejectsWideSpan(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{0, 0x20, 0, 0x41}
	body = append(body, sleb128(-0x80000000)...)
	body = append(body, 0x6a, 0x28, 2, 0, 0x1a, 0x20, 0, 0x28, 2, 0, 0x0b)
	m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
	m.Memories[0].Limits.HasMax = true
	m.Memories[0].Limits.Max = 1
	var stats ModuleStats
	_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"wrapped-load-group": true, "signed-load-group": true}}, nil, 0x80000000)
	if err == nil {
		t.Fatal("second load did not trap")
	}
	if stats.Funcs[0].Peephole["wrapped-load-group"] != 0 {
		t.Fatal("signed displacement overflow admitted")
	}
}
