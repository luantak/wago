//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSelectFlagsSinkInvalidatesBounds(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	body := []byte{1, 6, 0x7f,
		0x20, 0, 0x28, 2, 0, 0x1a,
		0x41, 0, 0x41, 42, 0x36, 2, 0,
		0x20, 0, 0x20, 1, 0x20, 0, 0x20, 1, 0x4b, 0x1b, 0x21, 0,
		0x20, 0, 0x28, 2, 0, 0x1a, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, nil, body)
	for _, address := range []uint64{0, 4, 65532, 65533, 65536, 0xffffffff} {
		var baseline []byte
		for _, enabled := range []bool{false, true} {
			var stats ModuleStats
			_, memory, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-local-sink": true, "select-flags-sink": enabled}}, nil, 4, address)
			if (err != nil) != (address > 65532) {
				t.Fatalf("enabled=%v address=%x trap=%v", enabled, address, err)
			}
			if enabled && stats.Funcs[0].Peephole["select-flags-sink"] == 0 {
				t.Fatal("fixture did not exercise select sink")
			}
			if memory[0] != 42 {
				t.Fatal("store before trap was lost")
			}
			if !enabled {
				baseline = memory
			} else if !bytes.Equal(baseline, memory) {
				t.Fatal("select sink changed memory")
			}
		}
	}
}

func TestSelectFlagsSink(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		ty, decl, cmp, add := wasm.I32, byte(0x7f), byte(0x4a), byte(0x6a)
		if wide {
			ty, decl, cmp, add = wasm.I64, 0x7e, 0x55, 0x7c
		}
		for _, tee := range []bool{false, true} {
			body := []byte{1, 6, decl, 0x20, 0, 0x20, 0, 0x20, 1, 0x20, 0, 0x20, 1, cmp, 0x1b}
			if tee {
				body = append(body, 0x22, 0)
			} else {
				body = append(body, 0x21, 0, 0x20, 0)
			}
			body = append(body, 0x20, 0, add, add, 0x0b)
			m := mod1(t, []wasm.ValType{ty, ty}, []wasm.ValType{ty}, body)
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-local-sink": true, "select-flags-sink": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				if (stats.Funcs[0].Peephole["select-flags-sink"] > 0) != enabled {
					t.Fatalf("enabled=%v stats=%v", enabled, stats.Funcs[0].Peephole)
				}
				for _, args := range [][2]uint64{{3, 11}, {11, 3}, {0xffffffff, 0x80000000}, {0xffffffffffffffff, 0xdeadbeef01234567}} {
					a, b := int64(args[0]), int64(args[1])
					if !wide {
						a, b = int64(int32(a)), int64(int32(b))
					}
					v := max(a, b)
					want := args[0] + 2*uint64(v)
					if !wide {
						want = uint64(uint32(want))
					}
					if got := runCompiledAmd64u(t, cm, args[:]...); got != want {
						t.Fatalf("wide=%v tee=%v args=%x got=%x want=%x", wide, tee, args, got, want)
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}
