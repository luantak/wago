//go:build linux && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestBulkReadyArgs(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, older := range []bool{false, true} {
		body := []byte{0}
		if older {
			body = append(body, 0x41, 7)
		}
		body = append(body, 0x20, 0, 0x20, 1, 0x20, 2, 0xfc, 10, 0, 0)
		if !older {
			body = append(body, 0x41, 7)
		}
		body = append(body, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, on := range []bool{false, true} {
			for _, pair := range [][2]uint64{{100, 200}, {200, 100}, {100, 100}, {100, 101}, {101, 100}, {65530, 100}, {100, 65530}, {65536, 65536}, {0xffffffff, 0}} {
				for _, n := range []uint64{0, 1, 7, 15, 16, 17, 31, 32, 33, 47, 48, 49, 255, 256, 513} {
					want := make([]byte, 65536)
					for i := range want {
						want[i] = byte(i*17 + 3)
					}
					trap := pair[0]+n > 65536 || pair[1]+n > 65536
					if !trap {
						copy(want[pair[0]:pair[0]+n], want[pair[1]:pair[1]+n])
					}
					var stats ModuleStats
					got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"bulk-ready-args": on}}, func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*17 + 3)
						}
					}, pair[0]|0xdeadbeef00000000, pair[1]|0xcafebabe00000000, n|0xffff000000000000)
					if (err != nil) != trap {
						t.Fatalf("older=%v on=%v pair=%v n=%d err=%v", older, on, pair, n, err)
					}
					// A trapping bulk copy must not partially modify the destination.
					if !bytes.Equal(mem, want) || (!trap && got != 7) {
						t.Fatalf("bad copy older=%v on=%v pair=%v n=%d", older, on, pair, n)
					}
					if (stats.Funcs[0].Peephole["bulk-ready-args"] > 0) != (on && !older) {
						t.Fatalf("admission older=%v on=%v stats=%v", older, on, stats.Funcs[0].Peephole)
					}
				}
			}
		}
	}
}
