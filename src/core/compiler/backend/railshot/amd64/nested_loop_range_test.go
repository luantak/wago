//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func nestedRangePolicy(t *testing.T, on bool) CodegenPolicy {
	t.Helper()
	sel, err := optimizationBindings.ResolveSnapshot(map[string]bool{"structured-loop-range": true, "nested-loop-range": on}, OptimizationSnapshot{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return shared.DefaultCodegenPolicy(sel)
}

func TestNestedLoopRangeProof(t *testing.T) {
	for _, on := range []bool{false, true} {
		f := fn{policy: nestedRangePolicy(t, on), nLocals: 2, localType: []machineType{mtI32, mtI32}, locals: []localDef{{typ: mtI32, state: lsConstZero}, {typ: mtI32, state: lsConstZero}}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		body := []byte{0x03, 0x40, 0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x41, 4, 0x47, 0x0d, 0, 0x0b, 0x20, 0, 0x41, 1, 0x6a, 0x22, 0, 0x41, 4, 0x47, 0x0d, 0, 0x0b}
		local, maximum := f.planStructuredLoopRange(wasm.NewReader(body))
		if on && (local != 1 || maximum != 4) || !on && local != 0 {
			t.Fatalf("on=%v local=%d maximum=%d", on, local, maximum)
		}
	}
}

func TestNestedLoopRangeRejectsEscapesAndOuterWrites(t *testing.T) {
	for _, shape := range []string{"continue-outer", "extra-outer-write", "unknown-entry", "call"} {
		f := fn{policy: nestedRangePolicy(t, true), nLocals: 2, localType: []machineType{mtI32, mtI32}, locals: []localDef{{typ: mtI32, state: lsConstZero}, {typ: mtI32, state: lsConstZero}}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		body := []byte{0x03, 0x40}
		switch shape {
		case "continue-outer":
			body = append(body, 0x41, 1, 0x0d, 1)
		case "extra-outer-write":
			body = append(body, 0x41, 9, 0x21, 0)
		case "unknown-entry":
			f.locals[0].state = lsMem
		case "call":
			body = append(body, 0x10, 0)
		}
		body = append(body, 0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x41, 4, 0x47, 0x0d, 0, 0x0b, 0x20, 0, 0x41, 1, 0x6a, 0x22, 0, 0x41, 4, 0x47, 0x0d, 0, 0x0b)
		if local, _ := f.planStructuredLoopRange(wasm.NewReader(body)); local != 0 {
			t.Fatalf("admitted%s", shape)
		}
	}
}

func TestNestedLoopRangeStack(t *testing.T) {
	f := fn{policy: nestedRangePolicy(t, true), sc: newScratch(), s: newStack(), ctrl: []ctrlFrame{{kind: cfBlock}}, localType: []machineType{mtI32, mtI32, mtI32}}
	for i := 0; i < 3; i++ {
		f.ctrl = append(f.ctrl, ctrlFrame{kind: cfLoop})
		f.activateLoopAddressRange(uint32(i+1), uint64(4<<i))
	}
	for i := 2; i >= 0; i-- {
		// A control merge drops straight-line facts, but cannot invalidate a
		// monotone outer-loop proof that covers this entire nested region.
		f.invalidateBoundsCert()
		for j := 0; j <= i; j++ {
			e := f.pushValue(storage{kind: stLocalRef, typ: mtI32, idx: uint32(j)})
			budget := 32
			if maximum := f.unsignedAddressMaximum(e, false, &budget); maximum != uint64(4<<j) {
				t.Fatalf("depth=%d local=%d max=%d", i, j, maximum)
			}
		}
		last := len(f.ctrl) - 1
		f.loopAddressLocal, f.loopAddressMaximum, f.loopAddressDepth = 0, 0, 0
		f.restoreEnclosingLoopAddressRange(last)
		f.ctrl = f.ctrl[:last]
	}
	if f.loopAddressLocal != 0 || f.loopAddressDepth != 0 {
		t.Fatal("loop range escaped its control frame")
	}
}

func TestNestedLoopRangeDoesNotOverwritePinPlan(t *testing.T) {
	f := fn{policy: nestedRangePolicy(t, true), sc: newScratch(), ctrl: []ctrlFrame{{kind: cfBlock}, {kind: cfLoop, flags: ctrlLoopPinExchange}}, loopAddressLocal: 1, loopAddressMaximum: 4, loopAddressDepth: 2}
	f.ensureCtrlMerge(&f.ctrl[1]).loopPinPlan = 0x12345678
	f.ctrl = append(f.ctrl, ctrlFrame{kind: cfLoop})
	if f.activateLoopAddressRange(2, 8) || f.loopAddressLocal != 1 || f.loopAddressDepth != 2 || f.ctrlMerge(&f.ctrl[1]).loopPinPlan != 0x12345678 {
		t.Fatal("nested range collided with pin exchange state")
	}
}

func TestNestedLoopRangeFlagsPreserveFloatBase(t *testing.T) {
	for base := 0; base < 4; base++ {
		fr := ctrlFrame{flags: ctrlLoopSavedRange | ctrlLoopOuterRange | ctrlHasElse}
		fr.setFloatConstBase(base)
		if fr.floatConstBase() != base || !fr.has(ctrlLoopSavedRange) || !fr.has(ctrlLoopOuterRange) || !fr.has(ctrlHasElse) {
			t.Fatalf("base=%d flags=%x", base, fr.flags)
		}
	}
}
