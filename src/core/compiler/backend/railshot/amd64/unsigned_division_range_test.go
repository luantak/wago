//go:build linux && amd64

package amd64

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
)

func TestUnsignedRemainderRangeNative(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, remainder := range []bool{false, true} {
		d, op := uint32(65537), byte(0x6e)
		if remainder {
			d, op = 1000, 0x70
		}
		for _, materialize := range []bool{false, true} {
			for _, on := range []bool{false, true} {
				body := []byte{1, 1, 0x7f, 0x20, 0, 0x41}
				body = append(body, wasmtest.SLEB32(int32(d))...)
				body = append(body, op)
				if materialize {
					body = append(body, 0x21, 1, 0x01, 0x20, 1)
				}
				body = append(body, 0x41, 7, 0x6e, 0x20, 0, 0x73, 0x0b)
				m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"unsigned-remainder-range": on}})
				if err != nil {
					t.Fatal(err)
				}
				defer cm.CodeImage.Close()
				if (stats.Funcs[0].Peephole["bounded-unsigned-div"] != 0) != (remainder && on) {
					t.Fatalf("remainder=%v materialize=%v missing bounded lowering: %v", remainder, materialize, stats.Funcs[0].Peephole)
				}
				for _, input := range []uint64{0, 1, 999, 1000, 65536, 65537, 0x7fffffff, 0xffffffff, 0xfedcba9876543210} {
					n := uint32(input)
					v := n / d
					if remainder {
						v = n % d
					}
					want := v/7 ^ n
					if got := runCompiledAmd64u(t, cm, input); got != uint64(want) {
						t.Fatalf("remainder=%v materialize=%v n=%x got=%x want=%x", remainder, materialize, input, got, want)
					}
				}
			}
		}
	}
}

func TestUnsignedRemainderRangeMemoryAndTraps(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		typ, constant, rem, localType := wasm.I32, byte(0x41), byte(0x70), byte(0x7f)
		if wide {
			typ, constant, rem, localType = wasm.I64, 0x42, 0x82, 0x7e
		}
		for _, divisor := range []int32{0, 1000, 65536, 65537} {
			for _, mutation := range []string{"none", "overwrite", "join"} {
				for _, offset := range []byte{0, 1} {
					// The preceding store must survive a remainder or load trap.
					body := []byte{1, 1, localType, constant, 8, 0x41, 42, 0x36, 0, 0, 0x20, 0, constant}
					body = append(body, wasmtest.SLEB32(divisor)...)
					body = append(body, rem, 0x21, 2, 0x01)
					if mutation == "overwrite" {
						body = append(body, 0x20, 0, 0x21, 2)
					}
					if mutation == "join" {
						body = append(body, 0x20, 1, 0x04, 0x40, 0x20, 0, 0x21, 2, 0x0b)
					}
					body = append(body, 0x20, 2, 0x28, 0, offset, 0x0b)
					m := modMem(t, 1, []wasm.ValType{typ, wasm.I32}, []wasm.ValType{wasm.I32}, body)
					m.Memories[0].Limits.Addr64 = wide
					for _, input := range []uint64{0, 999, 65532, 65535, 65536, ^uint64(0)} {
						for _, condition := range []uint64{0, 1} {
							init := func(mem []byte) {
								for i := range mem {
									mem[i] = byte(i*13 + 7)
								}
							}
							var beforeStats ModuleStats
							want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &beforeStats, Optimizations: map[string]bool{"unsigned-remainder-range": false}}, init, input, condition)
							var stats ModuleStats
							got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"unsigned-remainder-range": true}}, init, input, condition)
							if fmt.Sprint(we) != fmt.Sprint(ge) || we == nil && want != got || !bytes.Equal(wm, gm) {
								t.Fatalf("wide=%v d=%d mutation=%s offset=%d x=%x cond=%d got=%x/%v want=%x/%v", wide, divisor, mutation, offset, input, condition, got, ge, want, we)
							}
							if (stats.Funcs[0].Peephole["memory-min-bounds"] > beforeStats.Funcs[0].Peephole["memory-min-bounds"]) != (mutation == "none" && divisor == 1000) {
								t.Fatalf("unexpected memory proof wide=%v d=%d mutation=%s offset=%d: %v", wide, divisor, mutation, offset, stats.Funcs[0].Peephole)
							}
						}
					}
				}
			}
		}
	}
}
