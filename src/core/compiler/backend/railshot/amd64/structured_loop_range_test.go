//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func BenchmarkStructuredLoopRangeUnknownStart(b *testing.B) {
	saved := structuredLoopRangeEnabled
	structuredLoopRangeEnabled = true
	defer func() { structuredLoopRangeEnabled = saved }()
	f := fn{policy: currentCodegenPolicy(), nLocals: 1, localType: []machineType{mtI32}, locals: []localDef{{typ: mtI32, reg: regNone, state: lsMem}}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	body := make([]byte, 0, 256)
	for range 80 {
		body = append(body, 0x41, 1, 0x1a)
	}
	body = append(body, 0x20, 0, 0x41, 1, 0x6a, 0x22, 0, 0x41, 16, 0x47, 0x0d, 0, 0x0b)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if local, _ := f.planStructuredLoopRange(wasm.NewReader(body)); local != 0 {
			b.Fatal("unknown starting value admitted")
		}
	}
}

func TestStructuredLoopRangeProof(t *testing.T) {
	saved := structuredLoopRangeEnabled
	structuredLoopRangeEnabled = true
	defer func() { structuredLoopRangeEnabled = saved }()
	f := fn{policy: currentCodegenPolicy(), nLocals: 1, localType: []machineType{mtI32}, locals: []localDef{{typ: mtI32, reg: regNone, state: lsMem}}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	f.rememberAddressMaximum(0, 0)
	body := []byte{0x20, 0, 0x04, 0x40, 0x41, 1, 0x1a, 0x05, 0x41, 2, 0x1a, 0x0b, 0x20, 0, 0x41, 1, 0x6a, 0x22, 0, 0x41, 16, 0x47, 0x0d, 0, 0x0b}
	if local, max := f.planStructuredLoopRange(wasm.NewReader(body)); local != 1 || max != 16 {
		t.Fatalf("local=%d max=%d", local, max)
	}
}

func TestStructuredLoopRangeLazyZeroStart(t *testing.T) {
	saved := structuredLoopRangeEnabled
	structuredLoopRangeEnabled = true
	defer func() { structuredLoopRangeEnabled = saved }()
	f := fn{policy: currentCodegenPolicy(), nLocals: 1, localType: []machineType{mtI32}, locals: []localDef{{typ: mtI32, reg: regNone, state: lsConstZero}}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	body := []byte{0x20, 0, 0x41, 1, 0x6a, 0x22, 0, 0x41, 16, 0x47, 0x0d, 0, 0x0b}
	if local, max := f.planStructuredLoopRange(wasm.NewReader(body)); local != 1 || max != 16 {
		t.Fatalf("local=%d max=%d", local, max)
	}
}

func TestStructuredLoopRangeRejectsUnsafe(t *testing.T) {
	saved := structuredLoopRangeEnabled
	structuredLoopRangeEnabled = true
	defer func() { structuredLoopRangeEnabled = saved }()
	for _, shape := range []string{"unknown", "extra-write", "inner-branch", "unreachable-stride", "overflow", "inline"} {
		f := fn{policy: currentCodegenPolicy(), nLocals: 1, localType: []machineType{mtI32}, locals: []localDef{{typ: mtI32, reg: regNone, state: lsMem}}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		if shape != "unknown" {
			f.rememberAddressMaximum(0, 0)
		}
		body := []byte{0x20, 0, 0x04, 0x40}
		if shape == "extra-write" {
			body = append(body, 0x41, 1, 0x21, 0)
		}
		if shape == "inner-branch" {
			body = append(body, 0x41, 1, 0x0d, 1) // escapes the if to continue the outer loop
		}
		body = append(body, 0x0b, 0x20, 0, 0x41)
		step := byte(1)
		if shape == "unreachable-stride" {
			step = 3
		}
		if shape == "overflow" {
			step = 2
		}
		body = append(body, step, 0x6a, 0x22, 0, 0x41)
		if shape == "overflow" {
			body = append(body, 0xff, 0xff, 0xff, 0xff, 7)
		} else {
			body = append(body, 16)
		}
		body = append(body, 0x47, 0x0d, 0, 0x0b)
		if shape == "inline" {
			f.localBase = 1
		}
		if local, _ := f.planStructuredLoopRange(wasm.NewReader(body)); local != 0 {
			t.Fatalf("admitted %s", shape)
		}
	}
}
