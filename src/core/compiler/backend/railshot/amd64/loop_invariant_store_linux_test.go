//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
	"testing"
)

func TestInvariantStorePrototypeNative(t *testing.T) {
	body := []byte{0, 0x02, 0x40, 0x20, 2, 0x41, 32, 0x46, 0x0d, 0, 0x03, 0x40}
	body = append(body, invariantStoreLoopBody()...)
	body = append(body, 0x0b, 0x20, 3, 0x20, 2, 0x73, 0x20, 1, 0x73, 0x0b)
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	before := append([]byte{}, m.Code[0].BodyBytes...)
	optimized, n := prototypeInvariantStoreModule(m)
	if n != 1 {
		t.Fatalf("rewrites=%d want1", n)
	}
	if !bytes.Equal(before, m.Code[0].BodyBytes) {
		t.Fatal("source module mutated")
	}
	if err := wasm.ValidateModule(optimized); err != nil {
		t.Fatalf("rewritten module invalid: %v", err)
	}
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*19 + 23)
		}
	}
	opts := CompileOptions{Optimizations: map[string]bool{"loop-invariant-store": false}}
	on := CompileOptions{Optimizations: map[string]bool{"loop-invariant-store": true}}
	for _, count := range []uint64{0, 1, 2, 7, 8, 9, 16, 32} {
		for _, base := range []uint64{0, 1, 128, 65408, 65412, 65500, 65532, 0xfffffff0} {
			for _, dst := range []uint64{0, 1, 128, 512, 65532, 65533, 0xfffffffc} {
				args := []uint64{dst, base, 32 - count, 0xf1234567}
				want, wm, we := runMemAmd64WithOptions(t, m, opts, init, args...)
				got, gm, ge := runMemAmd64WithOptions(t, optimized, opts, init, args...)
				if (we != nil) != (ge != nil) || we == nil && got != want || !bytes.Equal(wm, gm) {
					t.Fatalf("count=%d base=%x dst=%x got=%x/%v want=%x/%v memory=%v", count, base, dst, got, ge, want, we, bytes.Equal(wm, gm))
				}
				actual, am, ae := runMemAmd64WithOptions(t, m, on, init, args...)
				if (we != nil) != (ae != nil) || we == nil && actual != want || !bytes.Equal(wm, am) {
					t.Fatalf("proved load n=%d base=%x dst=%x got=%x/%v want=%x/%v memory=%v", count, base, dst, actual, ae, want, we, bytes.Equal(wm, am))
				}
			}
		}
	}
	old := invariantStoreAssertFast
	invariantStoreAssertFast = true
	defer func() { invariantStoreAssertFast = old }()
	forced, n := prototypeInvariantStoreModule(m)
	if n != 1 {
		t.Fatal("forced prefix absent")
	}
	for _, count := range []uint64{8, 9, 16, 32} {
		args := []uint64{512, 0, 32 - count, 0xf1234567}
		want, wm, we := runMemAmd64WithOptions(t, m, opts, init, args...)
		got, gm, ge := runMemAmd64WithOptions(t, forced, opts, init, args...)
		if we != nil || ge != nil || got != want || !bytes.Equal(wm, gm) {
			t.Fatalf("forced n=%d got=%x/%v want=%x/%v memory=%v", count, got, ge, want, we, bytes.Equal(wm, gm))
		}
		actual, am, ae := runMemAmd64WithOptions(t, m, on, init, args...)
		if ae != nil || actual != want || !bytes.Equal(am, wm) {
			t.Fatalf("forced proved-load prefix n=%d got=%x/%v want=%x", count, actual, ae, want)
		}
	}
	var stats ModuleStats
	on.Stats = optionalTestStats(&stats)
	cm, err := CompileModuleWith(m, on)
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if diagnosticsEnabled && stats.Funcs[0].Peephole["invariant-store-prefix-load"] == 0 {
		t.Fatal("prefix repeated load bounds checks")
	}
}

func TestInvariantStoreWrappedAddressesAndLatch(t *testing.T) {
	for _, unroll := range []bool{false, true} {
		t.Run(map[bool]string{false: "scalar", true: "unrolled"}[unroll], func(t *testing.T) {
			testInvariantStoreWrappedAddressesAndLatch(t, unroll)
		})
	}
}

func testInvariantStoreWrappedAddressesAndLatch(t *testing.T, unroll bool) {
	for _, step := range []byte{1, 2, 4} {
		for _, dead := range []bool{false, true} {
			loop := invariantStoreLoopBody()
			pos := bytes.Index(loop, []byte{0x20, 2, 0x41, 1, 0x6a})
			loop[pos+3] = step
			pos = bytes.Index(loop, []byte{0x20, 1, 0x28})
			loop = append(append(append([]byte{}, loop[:pos+2]...), 0x41, 16, 0x6a), loop[pos+2:]...)
			if dead {
				pos = bytes.Index(loop, []byte{0x36, 2, 0}) + 3
				extra := []byte{0x20, 1, 0x41}
				extra = append(extra, wasmtest.SLEB32(1024)...)
				extra = append(extra, 0x6a, 0x28, 2, 0, 0x1a)
				loop = append(append(append([]byte{}, loop[:pos]...), extra...), loop[pos:]...)
			}
			limit := []byte{0x20, 4, 0x41}
			limit = append(limit, wasmtest.SLEB32(-2147483648)...)
			limit = append(limit, 0x6a)
			pos = bytes.Index(loop, []byte{0x41, 32, 0x47})
			loop = append(append(append([]byte{}, loop[:pos]...), limit...), loop[pos+2:]...)
			body := []byte{0, 0x02, 0x40, 0x20, 2}
			body = append(body, limit...)
			body = append(body, 0x46, 0x0d, 0, 0x03, 0x40)
			body = append(body, loop...)
			body = append(body, 0x0b, 0x20, 3, 0x20, 2, 0x73, 0x20, 1, 0x73, 0x0b)
			m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
			if _, n := prototypeInvariantStoreModule(m); n != 1 {
				t.Fatal("variant not planned", step, dead)
			}
			init := func(mem []byte) {
				for i := range mem {
					mem[i] = byte(i*37 + 11)
				}
			}
			for _, target := range []uint32{0x80000040, 0xfffffff0} {
				for _, n := range []uint32{0, 1, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16} {
					for _, src := range []uint32{0, 65492, 65500, 0xfffffff8} {
						for _, dst := range []uint32{0, 8, 8192, 65532} {
							args := []uint64{uint64(dst) | 0xdeadbeef00000000, uint64(src) | 0x8765432100000000, uint64(target-uint32(step)*n) | 0xaabbccdd00000000, 0xfedcba9876543210, uint64(target^0x80000000) | 0x7654321000000000}
							want, wm, we := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"loop-invariant-store": false}}, init, args...)
							got, gm, ge := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"loop-invariant-store": true, "loop-invariant-store-unroll": unroll}}, init, args...)
							if (we != nil) != (ge != nil) || we == nil && got != want || !bytes.Equal(wm, gm) {
								t.Fatalf("step=%d dead=%v target=%x n=%d src=%x dst=%x got=%x/%v want=%x/%v memory=%v", step, dead, target, n, src, dst, got, ge, want, we, bytes.Equal(wm, gm))
							}
						}
					}
				}
			}
		}
	}
}

func TestInvariantStoreProofFunctionScopeAndModes(t *testing.T) {
	for _, unroll := range []bool{false, true} {
		t.Run(map[bool]string{false: "scalar", true: "unrolled"}[unroll], func(t *testing.T) {
			testInvariantStoreProofFunctionScopeAndModes(t, unroll)
		})
	}
}

func testInvariantStoreProofFunctionScopeAndModes(t *testing.T, unroll bool) {
	requireCompilerDiagnostics(t)
	body := []byte{0, 0x02, 0x40, 0x20, 2, 0x41, 32, 0x46, 0x0d, 0, 0x03, 0x40}
	body = append(body, invariantStoreLoopBody()...)
	body = append(body, 0x0b, 0x20, 3, 0x0b)
	params := []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}
	m := modMem(t, 1, params, []wasm.ValType{wasm.I32}, body)
	_, n, proof := invariantStoreModuleWithLoadPolicy(m, unroll)
	wantProofs := 1
	if unroll {
		wantProofs = 2*invariantStoreUnrollFactor - 1
	}
	if n != 1 || len(proof[0]) != wantProofs {
		t.Fatal("missing proof")
	}
	readonly := []byte{0}
	for i := uint32(3); i < proof[0][0]; i++ {
		readonly = append(readonly, 0x01)
	}
	readonly = append(readonly, 0x20, 0, 0x28, 2, 0, 0x0b)
	other := modMem(t, 1, params, []wasm.ValType{wasm.I32}, readonly)
	m.Code = append(m.Code, other.Code[0])
	m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 2} {
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Workers: workers, Stats: &stats, Optimizations: map[string]bool{"loop-invariant-store": true, "loop-invariant-store-unroll": unroll}})
		if err != nil {
			t.Fatal(err)
		}
		if cm.CodeImage != nil {
			cm.CodeImage.Close()
		}
		if stats.Funcs[0].Peephole["invariant-store-prefix-load"] != wantProofs || stats.Funcs[1].Peephole["invariant-store-prefix-load"] != 0 || stats.Funcs[1].BoundsChecks == 0 {
			t.Fatalf("workers=%d proof leaked: first=%v second=%v secondBounds=%d", workers, stats.Funcs[0].Peephole, stats.Funcs[1].Peephole, stats.Funcs[1].BoundsChecks)
		}
	}
	modes := []string{"interruptible", "compact"}
	if profileEnabled {
		modes = append(modes, "profile")
	}
	for _, mode := range modes {
		var stats ModuleStats
		opts := CompileOptions{Interruptible: mode == "interruptible", CompactNative: mode == "compact", Profile: mode == "profile", Stats: &stats, Optimizations: map[string]bool{"loop-invariant-store": false, "loop-invariant-store-unroll": unroll}}
		a, err := CompileModuleWith(m, opts)
		if err != nil {
			t.Fatal(err)
		}
		opts.Optimizations["loop-invariant-store"] = true
		b, err := CompileModuleWith(m, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Code, b.Code) {
			t.Fatal("excluded mode changed native image", mode)
		}
		if a.CodeImage != nil {
			a.CodeImage.Close()
		}
		if b.CodeImage != nil {
			b.CodeImage.Close()
		}
	}
}
