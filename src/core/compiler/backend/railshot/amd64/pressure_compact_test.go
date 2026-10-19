//go:build linux && amd64

package amd64

import (
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestPressureCompact(t *testing.T) {
	testPressureCompactionRule(t, "pressure-compact")
}

func testPressureCompactionRule(t *testing.T, option string) {
	requireCompilerDiagnostics(t)
	// Several independent pure trees remain live while later checked loads
	// acquire registers. Their temporary locals are overwritten between trees.
	body := []byte{1, 2, 0x7f}
	for i := byte(0); i < 12; i++ {
		body = append(body, 0x20, 0, 0x28, 2, i*4, 0x22, 1,
			0x20, 1, 0x41, 31, 0x75, 0x22, 2, 0x73, 0x20, 2, 0x6b)
	}
	for i := 0; i < 11; i++ {
		body = append(body, 0x6a)
	}
	body = append(body, 0x0b)
	m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
	values := []uint32{0, 1, 0xffffffff, 0x80000000, 0x7fffffff, 11, 0xfffffff0, 99, 2, 5, 17, 0xdeadbeef}
	var expected uint32
	for _, v := range values {
		if int32(v) < 0 {
			expected -= v
		} else {
			expected += v
		}
	}
	setup := func(mem []byte) {
		for i, v := range values {
			binary.LittleEndian.PutUint32(mem[i*4:], v)
		}
	}
	for _, enabled := range []bool{false, true} {
		for _, addr := range []uint64{0, 65532, 0xffffffff} {
			var stats ModuleStats
			got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{option: enabled}}, setup, addr)
			if addr == 0 {
				if err != nil || uint32(got) != expected {
					t.Fatalf("enabled=%v got=%x/%v want=%x", enabled, got, err, expected)
				}
			} else if err == nil {
				t.Fatalf("enabled=%v address=%x did not trap", enabled, addr)
			}
			if (stats.Funcs[0].Peephole[option] > 0) != enabled {
				t.Fatalf("admission enabled=%v: %v", enabled, stats.Funcs[0].Peephole)
			}
		}
	}
}
