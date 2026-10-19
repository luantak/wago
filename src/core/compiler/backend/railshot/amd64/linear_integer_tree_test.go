//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	x86 "github.com/wago-org/wago/src/core/encoder/amd64"
)

func TestLinearIntegerCollectionPressure(t *testing.T) {
	old := linearIntegerTreeEnabled
	linearIntegerTreeEnabled = true
	defer func() { linearIntegerTreeEnabled = old }()
	for _, kind := range []storageKind{stLocalReg, stLocalRef} {
		f := fn{a: &x86.Asm{}, s: newStack(), policy: currentCodegenPolicy(), hasLoop: true}
		for _, r := range gpAlloc {
			if r != RAX && r != RDX {
				f.reserved = f.reserved.add(r)
			}
		}
		for i := 0; i < 5; i++ {
			f.pushValue(storage{kind: kind, typ: mtI32, reg: RAX, idx: 0})
			f.pushValue(storage{kind: stConst, typ: mtI32, cval: 7})
			f.pushBinOp(opMul, mtI32)
			if i != 0 {
				f.pushBinOp(opAdd, mtI32)
			}
		}
		root := f.s.back()
		if got := f.tryLinearIntegerTree(root, regNone); got != regNone {
			t.Fatalf("pressure selected %v", got)
		}
		if len(f.a.B) != 0 || f.pinned != 0 || f.s.back() != root || !root.isDeferred() {
			t.Fatal("pressure fallback changed state")
		}
	}
}

func TestLinearIntegerTreeProofRejectsEffectsAndUnknownSources(t *testing.T) {
	for _, kind := range []storageKind{stReg, stSlot, stGlobalRef, stGlobReg, stMemRef} {
		f := fn{s: newStack(), policy: currentCodegenPolicy()}
		e := f.pushValue(storage{kind: kind, typ: mtI32})
		w := linearIntegerTree{f: &f, budget: 64}
		if w.collect(e, 0) {
			t.Fatalf("zero scale accepted observable/unknown kind %v", kind)
		}
	}
	for _, op := range []wOp{opDivS, opDivU, opRemS, opShrS, opShrU, opAnd, opMul} {
		f := fn{s: newStack(), policy: currentCodegenPolicy()}
		f.pushValue(storage{kind: stLocalRef, typ: mtI32, idx: 0})
		f.pushValue(storage{kind: stLocalRef, typ: mtI32, idx: 1})
		f.pushBinOp(op, mtI32)
		w := linearIntegerTree{f: &f, budget: 64}
		if w.collect(f.s.back(), 0) {
			t.Fatalf("zero scale accepted %v", op)
		}
	}
	f := fn{s: newStack(), policy: currentCodegenPolicy()}
	w := linearIntegerTree{f: &f, budget: 64}
	for i := uint32(0); i < 9; i++ {
		e := f.pushValue(storage{kind: stLocalRef, typ: mtI32, idx: i})
		if got := w.collect(e, 1); got != (i < 8) {
			t.Fatalf("index%d admitted=%t", i, got)
		}
	}
	w = linearIntegerTree{f: &f, budget: 0}
	if w.collect(f.s.back(), 1) {
		t.Fatal("exhausted proof admitted")
	}
}

func TestLinearIntegerCollection(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	// (7*x+11*y)*4 + (13*x-17*y) + 19*x = 60*x+27*y modulo 2^32.
	body := []byte{0, 0x03, 0x7f, 0x20, 0, 0x41, 7, 0x6c, 0x20, 1, 0x41, 11, 0x6c, 0x6a, 0x41, 2, 0x74, 0x20, 0, 0x41, 13, 0x6c, 0x20, 1, 0x41, 17, 0x6c, 0x6b, 0x6a, 0x20, 0, 0x41, 19, 0x6c, 0x6a, 0x0b, 0x0b}
	alias := append(append([]byte{}, body[:len(body)-2]...), 0x22, 0, 0x0b, 0x0b)
	frame := append([]byte{1, 65, 0x7f}, body[1:]...)
	for _, code := range [][]byte{body, alias, frame} {
		m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, code)
		for _, enabled := range []bool{false, true} {
			var stats ModuleStats
			c, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"linear-integer-collect": enabled, "wide-local-pins": false}})
			if err != nil {
				t.Fatal(err)
			}
			for _, x := range []uint64{0, 1, 0x7fffffff, 0x80000000, 0xffffffff} {
				for _, y := range []uint64{0, 1, 0xffffffff} {
					want := uint64(uint32(60*uint32(x) + 27*uint32(y)))
					if got := runCompiledAmd64u(t, c, x, y); got != want {
						t.Fatalf("enabled=%v x=%x y=%x got=%x want=%x", enabled, x, y, got, want)
					}
				}
			}
			c.CodeImage.Close()
			if (stats.Funcs[0].Peephole["linear-integer-tree"] > 0) != enabled {
				t.Fatalf("enabled=%v selection=%v", enabled, stats.Funcs[0].Peephole)
			}
		}
	}
}

func TestLinearIntegerCollectionCancelledTerms(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, coefficients := range [][4]int32{{7, -7, 11, -11}, {7, -6, 11, -11}, {7, -8, 11, -11}} {
		for _, constant := range []byte{0, 5} {
			body := []byte{0, 0x03, 0x7f}
			var total uint32
			for i, c := range coefficients {
				total += uint32(c)
				body = append(body, 0x20, 0, 0x41, byte(c)&0x7f, 0x6c)
				if i > 0 {
					body = append(body, 0x6a)
				}
			}
			body = append(body, 0x41, constant, 0x6a, 0x0b, 0x0b)
			m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
			for _, enabled := range []bool{false, true} {
				c, err := CompileModuleWith(m, CompileOptions{Optimizations: map[string]bool{"linear-integer-collect": enabled}})
				if err != nil {
					t.Fatal(err)
				}
				for _, x := range []uint64{0, 1, 0x80000000, 0xffffffff} {
					want := uint64(uint32(x)*total + uint32(constant))
					if got := runCompiledAmd64u(t, c, x); got != want {
						t.Fatalf("coefficients=%v constant=%d x=%x got=%x want=%x", coefficients, constant, x, got, want)
					}
				}
				c.CodeImage.Close()
			}
		}
	}
}
