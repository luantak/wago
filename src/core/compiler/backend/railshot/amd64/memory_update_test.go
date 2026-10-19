//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestMemoryUpdate(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		for _, op := range []byte{0x6a, 0x6b, 0x71, 0x72, 0x73} {
			for _, imm := range []int64{1, -1, 128, 0x7fffffff} {
				load, store, con, alu := byte(0x28), byte(0x36), byte(0x41), op
				if wide {
					load, store, con, alu = 0x29, 0x37, 0x42, op+0x12
				}
				body := []byte{0, 0x20, 0, 0x20, 0, load, 0, 3, con}
				body = append(body, sleb128(imm)...)
				body = append(body, alu, store, 0, 3, 0x41, 0, 0x0b)
				m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
				for _, base := range []uint64{0, 1, 65525, 65529, 65530, 0xffffffff} {
					init := func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*31 + 127)
						}
					}
					var stats ModuleStats
					want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"memory-update": false}}, init, base)
					got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"memory-update": true}}, init, base)
					if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
						t.Fatalf("wide=%v op=%x imm=%d base=%x got=%v want=%v", wide, op, imm, base, ge, we)
					}
					if stats.Funcs[0].Peephole["memory-update"] == 0 && !(op == 0x71 && imm == -1 || op == 0x72 && imm == -1 || op == 0x73 && imm == -1) {
						t.Fatalf("missing memory-update admission: wide=%v op=%x imm=%d stats=%v", wide, op, imm, stats.Funcs[0].Peephole)
					}
				}
			}
		}
	}
}

func TestMemoryUpdateRejectsDifferentAddress(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, differentLocal := range []bool{false, true} {
		dst, off := byte(0), byte(4)
		if differentLocal {
			dst, off = 1, 3
		}
		body := []byte{0, 0x20, dst, 0x20, 0, 0x28, 0, 3, 0x41, 1, 0x6a, 0x36, 0, off, 0x41, 0, 0x0b}
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, args := range [][2]uint64{{0, 8}, {1, 0}, {65529, 0}, {0, 65535}, {0xffffffff, 0}} {
			init := func(mem []byte) {
				for i := range mem {
					mem[i] = byte(i*17 + 93)
				}
			}
			var stats ModuleStats
			want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"memory-update": false}}, init, args[:]...)
			got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"memory-update": true}}, init, args[:]...)
			if (we != nil) != (ge != nil) || we == nil && want != got || !bytes.Equal(wm, gm) {
				t.Fatalf("differentLocal=%v args=%v got=%v want=%v", differentLocal, args, ge, we)
			}
			if stats.Funcs[0].Peephole["memory-update"] != 0 {
				t.Fatal("different address admitted")
			}
		}
	}
}
