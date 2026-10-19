//go:build amd64

package amd64

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestWeightedLoopPinExchange(t *testing.T) {
	old := weightedLoopPinExchangeEnabled
	defer func() { weightedLoopPinExchangeEnabled = old }()
	for _, enabled := range []bool{false, true} {
		weightedLoopPinExchangeEnabled = enabled
		for _, donorReads := range []int{1, 4, 6} {
			f := fn{policy: currentCodegenPolicy(), nLocals: 2, localType: []machineType{mtI32, mtI32}, locals: []localDef{{typ: mtI32, reg: R12, state: lsReg}, {typ: mtI32, reg: regNone, state: lsMem}}, pinnedLocals: []int{0}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
			body := bytes.Repeat([]byte{0x20, 1, 0x1a}, 6)
			body = append(body, bytes.Repeat([]byte{0x20, 0, 0x1a}, donorReads)...)
			body = append(body, 0x41, 0, 0x0d, 0, 0x0b)
			plan, _ := f.planLoopPinExchange(wasm.NewReader(body))
			want := enabled && donorReads == 1
			if (plan != 0) != want {
				t.Fatalf("enabled=%v donorReads=%d plan=%x wantExchange=%v", enabled, donorReads, plan, want)
			}
		}
	}
}

func TestExtendedLoopPinScan(t *testing.T) {
	old := extendedLoopPinScanEnabled
	defer func() { extendedLoopPinScanEnabled = old }()
	for _, enabled := range []bool{false, true} {
		extendedLoopPinScanEnabled = enabled
		f := fn{policy: currentCodegenPolicy(), nLocals: 2, localType: []machineType{mtI32, mtI32},
			locals:       []localDef{{typ: mtI32, reg: R12, state: lsReg}, {typ: mtI32, reg: regNone, state: lsMem}},
			pinnedLocals: []int{0}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		body := bytes.Repeat([]byte{0x20, 1, 0x1a}, 60)
		body = append(body, 0x41, 0, 0x0d, 0, 0x0b)
		plan, _ := f.planLoopPinExchange(wasm.NewReader(body))
		if (plan != 0) != enabled {
			t.Fatalf("enabled=%v plan=%x", enabled, plan)
		}
		// A late use of the old pin must still veto the whole exchange.
		body = append(append([]byte{}, body[:len(body)-5]...), 0x20, 0, 0x1a, 0x41, 0, 0x0d, 0, 0x0b)
		if plan, _ := f.planLoopPinExchange(wasm.NewReader(body)); plan != 0 {
			t.Fatal("missed late pin use")
		}
		body = append(bytes.Repeat([]byte{0x20, 1, 0x1a}, 130), 0x41, 0, 0x0d, 0, 0x0b)
		if plan, _ := f.planLoopPinExchange(wasm.NewReader(body)); plan != 0 {
			t.Fatal("unbounded scan")
		}
	}
}

func TestLoopPinExchangeIntegerWidths(t *testing.T) {
	for _, oldType := range []machineType{mtI32, mtI64} {
		for _, newType := range []machineType{mtI32, mtI64} {
			t.Run(fmt.Sprintf("%d-to-%d", oldType, newType), func(t *testing.T) {
				f := fn{
					nLocals: 2, localType: []machineType{oldType, newType},
					locals:       []localDef{{typ: oldType, reg: R12, state: lsReg}, {typ: newType, reg: regNone, state: lsMem}},
					pinnedLocals: []int{0}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true),
				}
				// The old local is inactive. The new one is repeatedly read,
				// independent of its arithmetic or the function's application.
				body := []byte{0x20, 1, 0x1a, 0x20, 1, 0x1a, 0x20, 1, 0x1a, 0x41, 0, 0x0d, 0, 0x0b}
				plan, written := f.planLoopPinExchange(wasm.NewReader(body))
				if plan != loopPinEntry(0, 1, R12) || written != 0 {
					t.Fatalf("plan=%x written=%x", plan, written)
				}
				// An active old local must never be evicted by this planner.
				body = append([]byte{0x20, 0, 0x1a}, body...)
				if plan, _ := f.planLoopPinExchange(wasm.NewReader(body)); plan != 0 {
					t.Fatal("evicted an active local")
				}
			})
		}
	}
}
