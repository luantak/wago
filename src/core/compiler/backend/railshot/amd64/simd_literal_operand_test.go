//go:build linux && amd64

package amd64

import (
	"bytes"
	"fmt"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSIMDLiteralOperand(t *testing.T) {
	requireCompilerDiagnostics(t)
	a := [16]byte{0xff, 0x80, 1, 7, 0, 0xff, 0, 0x80, 3, 9, 2, 6, 4, 5, 7, 8}
	b := [16]byte{1, 2, 3, 4, 0xff, 0, 0x80, 0x7f, 9, 8, 7, 6, 5, 4, 3, 2}
	for _, cached := range []bool{false, true} {
		m := mod1(t, nil, []wasm.ValType{wasm.V128}, v128BinaryBody(a, b, 81))
		var stats ModuleStats
		got, _, err := runMemAmd64V128WithOptions(t, m, nil, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"v128-const-cache": cached, "simd-literal-operand": true}})
		if err != nil || got != applyV128BooleanBytes(81, a, b) {
			t.Fatalf("cached=%v got=%x err=%v", cached, got, err)
		}
		if stats.Funcs[0].Peephole["simd-literal-operand"] != 1 {
			t.Fatalf("cached=%v literal operand not folded", cached)
		}
	}
}

func TestSIMDLiteralOperandDifferential(t *testing.T) {
	requireCompilerDiagnostics(t)
	a := [16]byte{0xff, 0x80, 1, 7, 0, 0xff, 0, 0x80, 3, 9, 2, 6, 4, 5, 7, 8}
	b := [16]byte{1, 2, 3, 4, 0xff, 0, 0x80, 0x7f, 9, 8, 7, 6, 5, 4, 3, 2}
	for sub := uint32(0); sub < 256; sub++ {
		if simdLiteralBinary(sub) == 0 {
			continue
		}
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/cache=%v", sub, cached), func(t *testing.T) {
				// The old local remains live after the operation, so destructive
				// reuse of a borrowed XMM register is observable in the result.
				body := []byte{1, 1, 0x7b}
				body = append(body, v128ConstBytes(a)...)
				body = append(body, 0x21, 0, 0x20, 0)
				body = append(body, v128ConstBytes(b)...)
				body = append(body, simdOp(sub)...)
				body = append(body, 0x20, 0, 0xfd, 81, 0x0b)
				m := mod1(t, nil, []wasm.ValType{wasm.V128}, body)
				var stats ModuleStats
				on := CompileOptions{Stats: &stats, Optimizations: map[string]bool{"v128-const-cache": cached, "simd-literal-operand": true}}
				got, _, err := runMemAmd64V128WithOptions(t, m, nil, on)
				want, _, oldErr := runMemAmd64V128WithOptions(t, m, nil, CompileOptions{Optimizations: map[string]bool{"v128-const-cache": cached, "simd-literal-operand": false}})
				if err != nil || oldErr != nil || got != want {
					t.Fatalf("got=%x want=%x errors=%v/%v", got, want, err, oldErr)
				}
				if stats.Funcs[0].Peephole["simd-literal-operand"] != 1 {
					t.Fatal("literal operand missing")
				}
				on.AMD64FeaturesSet = true
				on.Stats = new(ModuleStats)
				legacy, _, err := runMemAmd64V128WithOptions(t, m, nil, on)
				if err != nil || legacy != want || on.Stats.Funcs[0].Peephole["simd-literal-operand"] != 0 {
					t.Fatalf("non-AVX fallback=%x want=%x err=%v", legacy, want, err)
				}
			})
		}
	}
	for _, base := range []uint32{0, 65520, 65521, 0xffffffff} {
		// Guest loads must still execute, with their full 16-byte bounds check,
		// before applying the following bit mask.
		body := []byte{0, 0x41}
		body = append(body, sleb128(int64(int32(base)))...)
		body = append(body, 0xfd, 0, 4, 0)
		body = append(body, v128ConstBytes(b)...)
		body = append(body, 0xfd, 78, 0x0b)
		m := modMem(t, 1, nil, []wasm.ValType{wasm.V128}, body)
		var expected [16]byte
		var initialMem []byte
		var trapped bool
		for _, enabled := range []bool{false, true} {
			var stats ModuleStats
			got, mem, err := runMemAmd64V128WithOptions(t, m, func(mem []byte) {
				for i := range mem {
					mem[i] = byte(i * 37)
				}
			}, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"simd-literal-operand": enabled}})
			if !enabled {
				expected, initialMem, trapped = got, mem, err != nil
			} else if got != expected || !bytes.Equal(mem, initialMem) || (err != nil) != trapped {
				t.Fatalf("base=%x mismatched trap, result or memory", base)
			}
		}
		if trapped != (base > 65520) {
			t.Fatalf("base=%x trap=%v", base, trapped)
		}
	}
}
