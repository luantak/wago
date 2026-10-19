//go:build linux && amd64

package amd64

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestMemoryFreeLeafLeaseRestoresCallerBounds(t *testing.T) {
	requireCompilerDiagnostics(t)
	// Eight independent integer locals keep pressure on the GP bank. The
	// callee may reuse the memory-size register, but its caller must still
	// perform correct explicit bounds checks after it returns.
	body := []byte{1, 8, 0x7e}
	for i := byte(2); i < 10; i++ {
		body = append(body, 0x42, i, 0x21, i)
	}
	body = append(body, 0x02, 0x40, 0x03, 0x40, 0x20, 1, 0x45, 0x0d, 1)
	for i := byte(2); i < 10; i++ {
		body = append(body, 0x20, i, 0x20, 0, 0x85, 0x42, i, 0x7c, 0x21, i)
	}
	body = append(body, 0x20, 1, 0x41, 1, 0x6b, 0x21, 1, 0x0c, 0, 0x0b, 0x0b, 0x42, 0)
	for i := byte(2); i < 10; i++ {
		body = append(body, 0x20, i, 0x7c)
	}
	body = append(body, 0x0b)
	m := modFuncs(t,
		funcDef{[]wasm.ValType{wasm.I64, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I64}, []byte{0, 0x20, 0, 0x20, 1, 0x10, 1, 0x20, 2, 0x28, 2, 0, 0xad, 0x85, 0x0b}},
		funcDef{[]wasm.ValType{wasm.I64, wasm.I32}, []wasm.ValType{wasm.I64}, body})
	m.Memories = []wasm.MemType{{Limits: wasm.Limits{Min: 1}}}
	for _, on := range []bool{false, true} {
		for _, workers := range []int{1, 2} {
			for _, n := range []uint64{0, 1, 7} {
				for _, addr := range []uint64{0, 65532, 65533, 65536, 0xffffffff} {
					t.Run(fmt.Sprintf("on=%v/workers=%d/n=%d/addr=%x", on, workers, n, addr), func(t *testing.T) {
						const seed = uint64(0xdeadbeef80000001)
						var want uint64
						for i := uint64(2); i < 10; i++ {
							x := i
							for j := uint64(0); j < n; j++ {
								x = (x ^ seed) + i
							}
							want += x
						}
						var stats ModuleStats
						got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Workers: workers, Stats: &stats, Optimizations: map[string]bool{"inline": false, "memsize-regional-lease": on}}, func(mem []byte) {
							binary.LittleEndian.PutUint32(mem, 0x12345678)
							binary.LittleEndian.PutUint32(mem[65532:], 0x12345678)
						}, seed, n, addr)
						if (err != nil) != (addr > 65532) {
							t.Fatalf("bounds trap = %v", err)
						}
						if err == nil && got != want^0x12345678 {
							t.Fatalf("got=%x want=%x", got, want^0x12345678)
						}
						if (stats.Funcs[1].Peephole["memsize-regional-lease"] != 0) != on {
							t.Fatalf("callee lease: %v", stats.Funcs[1].Peephole)
						}
					})
				}
			}
		}
	}
}
