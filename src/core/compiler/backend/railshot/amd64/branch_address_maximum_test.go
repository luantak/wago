//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestBranchAddressMaximumNestedJoin(t *testing.T) {
	// An if arm may establish a bound, but the skipped arm does not. The
	// post-join load must retain its check even when compilation saw the bound.
	body := []byte{0, 0x20, 1, 0x04, 0x40, 0x02, 0x40,
		0x20, 0, 0x41, 0xff, 1, 0x4b, 0x0d, 0,
		0x20, 0, 0x28, 0, 0, 0x1a, 0x0b, 0x0b,
		0x20, 0, 0x28, 0, 0, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, on := range []bool{false, true} {
		for _, arm := range []uint64{0, 1} {
			for _, x := range []uint64{0, 255, 256, 65532, 65533, 65536, 0xffffffff} {
				_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"branch-address-maximum": on}}, nil, x, arm)
				if (err != nil) != (x > 65532) {
					t.Fatalf("on=%v arm=%d x=%x err=%v", on, arm, x, err)
				}
			}
		}
	}
}

func TestBranchAddressMaximum(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, cmp := range [][]byte{{0x41, 0xff, 1, 0x4b}, {0x41, 0x80, 2, 0x4f}} {
		for _, shape := range []string{"straight", "overwrite", "join"} {
			body := []byte{0, 0x02, 0x40, 0x20, 0}
			body = append(body, cmp...)
			body = append(body, 0x0d, 0)
			if shape == "overwrite" {
				body = append(body, 0x20, 1, 0x21, 0)
			}
			if shape == "join" {
				body = append(body, 0x0b)
			}
			body = append(body, 0x20, 0, 0x28, 0, 0)
			if shape != "join" {
				body = append(body, 0x0f, 0x0b, 0x41, 7)
			}
			body = append(body, 0x0b)
			m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
			for _, on := range []bool{false, true} {
				for _, x := range []uint64{0, 255, 256, 65536, 0xffffffff} {
					var stats ModuleStats
					opts := CompileOptions{Stats: &stats, Optimizations: map[string]bool{"branch-address-maximum": on}}
					got, _, err := runMemAmd64WithOptions(t, m, opts, nil, x, 65536)
					trap := shape == "overwrite" && x <= 255 || shape == "join" && x > 65532
					want := uint64(0)
					if shape != "join" && x > 255 {
						want = 7
					}
					if (err != nil) != trap || !trap && got != want {
						t.Fatalf("shape=%s on=%v x=%x got=%d err=%v", shape, on, x, got, err)
					}
					if (stats.Funcs[0].Peephole["memory-min-bounds"] > 0) != (on && shape == "straight") {
						t.Fatalf("admission shape=%s on=%v: %v", shape, on, stats.Funcs[0].Peephole)
					}
				}
			}
		}
	}
}
