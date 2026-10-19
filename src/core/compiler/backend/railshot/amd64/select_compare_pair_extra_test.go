//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
)

func selectComparePairOracle(op int, a, b uint64, wide bool) bool {
	if !wide {
		a, b = uint64(uint32(a)), uint64(uint32(b))
	}
	sa, sb := int64(a), int64(b)
	if !wide {
		sa, sb = int64(int32(a)), int64(int32(b))
	}
	switch op {
	case 0:
		return a == b
	case 1:
		return a != b
	case 2:
		return sa < sb
	case 3:
		return a < b
	case 4:
		return sa > sb
	case 5:
		return a > b
	case 6:
		return sa <= sb
	case 7:
		return a <= b
	case 8:
		return sa >= sb
	case 9:
		return a >= b
	}
	panic("bad relation")
}

func TestSelectComparePairRelationsAndPressure(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		typ, first, xor, constant := wasm.I32, byte(0x46), byte(0x73), byte(0x41)
		if wide {
			typ, first, xor, constant = wasm.I64, 0x51, 0x85, 0x42
		}
		for lhs := 0; lhs < 10; lhs++ {
			for rhs := 0; rhs < 10; rhs++ {
				body := []byte{0}
				for i := 0; i < 20; i++ {
					// Select is eager: these are twenty owned values, not borrowed
					// local references or deferred trees that need no registers.
					body = append(body, 0x20, 0, constant, 0, 0x41, 1, 0x1b)
				}
				body = append(body, 0x20, 0, 0x20, 1, first+byte(lhs), 0x20, 0, 0x20, 1, first+byte(rhs), 0x20, 2, 0x1b)
				if wide {
					body = append(body, 0xad)
				}
				for i := 0; i < 20; i++ {
					body = append(body, xor)
				}
				body = append(body, 0x0b)
				var stats ModuleStats
				cm, err := CompileModuleWith(mod1(t, []wasm.ValType{typ, typ, wasm.I32}, []wasm.ValType{typ}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-compare-pair": true, "local-spill-reuse": true}})
				if err != nil {
					t.Fatal(err)
				}
				if stats.Funcs[0].Peephole["select-compare-pair"] == 0 {
					t.Fatalf("wide=%v lhs=%d rhs=%d not admitted", wide, lhs, rhs)
				}
				if stats.Funcs[0].Spills == 0 {
					t.Fatal("fixture did not create register pressure")
				}
				for _, a := range []uint64{0, 0xffffffff, 0xdeadbeef80000000, 1 << 63, ^uint64(0)} {
					for _, b := range []uint64{0, 7, 1 << 63, ^uint64(0)} {
						for _, condition := range []uint64{0, 1, 0x100000000, 0x80000000} {
							op := lhs
							if uint32(condition) == 0 {
								op = rhs
							}
							want := uint64(0)
							if selectComparePairOracle(op, a, b, wide) {
								want = 1
							}
							if got := runCompiledAmd64u(t, cm, a, b, condition); got != want {
								t.Fatalf("wide=%v lhs=%d rhs=%d args=%x/%x/%x got=%x want=%x", wide, lhs, rhs, a, b, condition, got, want)
							}
						}
					}
				}
				cm.CodeImage.Close()
			}
		}
	}
}

func TestSelectComparePairTeeAndMutation(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, mutate := range []bool{false, true} {
		body := []byte{0, 0x20, 0, 0x20, 1, 0x20, 2, 0x1b, 0x22, 0, 0x20, 1, 0x4c}
		if mutate {
			body = append(body, 0x41, 42, 0x21, 0)
		}
		body = append(body, 0x20, 0, 0x20, 1, 0x4e, 0x20, 2, 0x1b, 0x0b)
		for _, on := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-compare-pair": on, "local-spill-reuse": true, "owned-pinned-tee": true}})
			if err != nil {
				t.Fatal(err)
			}
			if (stats.Funcs[0].Peephole["select-compare-pair"] > 0) != (on && !mutate) {
				t.Fatalf("on=%v mutate=%v stats=%v", on, mutate, stats.Funcs[0].Peephole)
			}
			for _, a := range []uint64{0, 7, 0xdeadbeef80000000, 0xffffffff} {
				for _, b := range []uint64{0, 42, 0xffffffff} {
					for _, c := range []uint64{0, 1, 0x80000000, 0x100000000} {
						x := uint32(a)
						if uint32(c) == 0 {
							x = uint32(b)
						}
						y := x
						if mutate {
							y = 42
						}
						want := uint64(0)
						if uint32(c) != 0 && int32(x) <= int32(b) || uint32(c) == 0 && int32(y) >= int32(b) {
							want = 1
						}
						if got := runCompiledAmd64u(t, cm, a, b, c); got != want {
							t.Fatalf("mutate=%v on=%v args=%x/%x/%x got=%d want=%d", mutate, on, a, b, c, got, want)
						}
					}
				}
			}
			cm.CodeImage.Close()
		}
	}
}

func TestSelectComparePairWideImmediate(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{0, 0x20, 0, 0x42}
	body = append(body, wasmtest.SLEB64(1<<40)...)
	body = append(body, 0x57, 0x20, 0, 0x42)
	body = append(body, wasmtest.SLEB64(1<<40)...)
	body = append(body, 0x59, 0x20, 1, 0x1b, 0x0b)
	var stats ModuleStats
	cm, err := CompileModuleWith(mod1(t, []wasm.ValType{wasm.I64, wasm.I32}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"select-compare-pair": true, "local-spill-reuse": true}})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if stats.Funcs[0].Peephole["select-compare-pair"] == 0 {
		t.Fatal("wide immediate not admitted")
	}
	for _, v := range []uint64{0, 1<<40 - 1, 1 << 40, 1<<40 + 1, 1 << 63, ^uint64(0)} {
		for _, c := range []uint64{0, 1} {
			want := uint64(0)
			if c != 0 && int64(v) <= 1<<40 || c == 0 && int64(v) >= 1<<40 {
				want = 1
			}
			if got := runCompiledAmd64u(t, cm, v, c); got != want {
				t.Fatalf("v=%x c=%d got=%d want=%d", v, c, got, want)
			}
		}
	}
}
