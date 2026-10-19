//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	encoder "github.com/wago-org/wago/src/core/encoder/amd64"
)

func TestOwnedPinnedTeeOwnership(t *testing.T) {
	saved := ownedPinnedTeeEnabled
	ownedPinnedTeeEnabled = true
	defer func() { ownedPinnedTeeEnabled = saved }()
	for _, typ := range []machineType{mtI32, mtI64} {
		f := &fn{a: &encoder.Asm{}, s: newStackWithCap(8), stats: new(CodegenStats), policy: currentCodegenPolicy(),
			localType: []machineType{typ}, locals: []localDef{{typ: typ, reg: R12, state: lsMem}}}
		e := f.pushReg(RAX, typ)
		e.st.cval = 1
		if !f.keepOwnedPinnedTee(e, 0, R12, true) {
			t.Fatal("owned tee was not retained")
		}
		wantCopy := int64(0)
		if f.opt(optSelectComparePair) && f.opt(optLocalSpillReuse) {
			wantCopy = 1
		}
		if e.st.kind != stReg || e.st.reg != RAX || e.st.cval != wantCopy || f.regUser[RAX] != e ||
			f.regUser[R12] != nil || f.locals[0].state != lsReg {
			t.Fatalf("ownership or local state changed incorrectly: %+v", e.st)
		}
		if f.keepOwnedPinnedTee(e, 0, R12, false) {
			t.Fatal("non-tee assignment admitted")
		}
	}
}

func TestOwnedPinnedTee(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	// Reading through the old local before assigning the loaded value forces
	// an independent owned value. The first tee must survive the second tee.
	body := []byte{0,
		0x20, 0, 0x28, 2, 0, 0x22, 0,
		0x20, 0, 0x41, 31, 0x75, 0x22, 0,
		0x73, 0x20, 0, 0x6b, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, enabled := range []bool{false, true} {
		for _, value := range []uint32{0, 1, 0x7fffffff, 0x80000000, 0xffffffff, 0x87654321} {
			var stats ModuleStats
			got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats,
				Optimizations: map[string]bool{"owned-pinned-tee": enabled}}, func(mem []byte) {
				for i := range 4 {
					mem[16+i] = byte(value >> (8 * i))
				}
			}, 16)
			want := value
			if int32(value) < 0 {
				want = -value
			}
			if err != nil || got != uint64(want) {
				t.Fatalf("enabled=%v value=%x got=%x want=%x err=%v", enabled, value, got, want, err)
			}
			if enabled && stats.Funcs[0].Peephole["owned-pinned-tee"] == 0 {
				t.Fatal("fixture did not exercise owned pinned tee")
			}
		}
	}
}
