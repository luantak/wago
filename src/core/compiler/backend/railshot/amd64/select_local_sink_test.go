//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestSelectLocalSinkInvalidatesBounds(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	// Establish a bounds fact, make an observable store, then select a new
	// address into the same local. Its old bounds fact must not survive.
	body := []byte{1, 6, 0x7f,
		0x20, 0, 0x28, 2, 0, 0x1a,
		0x41, 0, 0x41, 42, 0x36, 2, 0,
		0x20, 1, 0x20, 0, 0x20, 2, 0x1b, 0x21, 0,
		0x20, 0, 0x28, 2, 0, 0x1a, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, nil, body)
	for _, address := range []uint64{4, 65532, 65533, 65536, 0xffffffff} {
		for _, condition := range []uint64{0, 1, 0x80000000} {
			var baseline []byte
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				_, memory, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-local-sink": enabled}}, nil, 4, address, condition)
				if (err != nil) != (condition != 0 && address > 65532) {
					t.Fatalf("enabled=%v address=%x condition=%x trap=%v", enabled, address, condition, err)
				}
				if enabled && stats.Funcs[0].Peephole["select-local-sink"] == 0 {
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
}

func TestSelectLocalSink(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		ty, decl, add := wasm.I32, byte(0x7f), byte(0x6a)
		if wide {
			ty, decl, add = wasm.I64, 0x7e, 0x7c
		}
		for _, copies := range []int{1, 4, 9} {
			body := []byte{1, 6, decl}
			for range copies {
				body = append(body, 0x20, 0)
			}
			body = append(body, 0x20, 1, 0x20, 0, 0x20, 2, 0x1b, 0x21, 0, 0x20, 0)
			for range copies {
				body = append(body, add)
			}
			body = append(body, 0x0b)
			m := mod1(t, []wasm.ValType{ty, ty, wasm.I32}, []wasm.ValType{ty}, body)
			for _, enabled := range []bool{false, true} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-local-sink": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				if copies == 1 && (stats.Funcs[0].Peephole["select-local-sink"] > 0) != enabled {
					t.Fatalf("enabled=%v stats=%v", enabled, stats.Funcs[0].Peephole)
				}
				for _, args := range [][3]uint64{{3, 11, 0}, {3, 11, 1}, {0xffffffff, 0x80000000, 17}, {0xffffffffffffffff, 0xdeadbeef01234567, 0}, {0xffffffffffffffff, 0xdeadbeef01234567, 0x80000000}} {
					want := args[0]
					if uint32(args[2]) != 0 {
						want = args[1]
					}
					want += uint64(copies) * args[0]
					if !wide {
						want = uint64(uint32(want))
					}
					if got := runCompiledAmd64u(t, cm, args[:]...); got != want {
						t.Fatalf("wide=%v copies=%d enabled=%v args=%x got=%x want=%x", wide, copies, enabled, args, got, want)
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}
