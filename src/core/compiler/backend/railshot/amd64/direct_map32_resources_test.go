//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// Untimed measurement of actual staging/mapping bytes, rather than Go B/op.
func TestDirectMap32CapacityResources(t *testing.T) {
	if !diagnosticsEnabled {
		t.Skip("requires wago_codegenstats")
	}
	for _, name := range []string{"i32", "f32", "rejected", "false-positive", "false-page", "small", "control"} {
		for _, deferred := range []bool{false, true} {
			m := directMap32Fixture(t, name == "f32", name == "rejected")
			if name == "false-positive" {
				m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x20, 4, 0x6c}, []byte{0x20, 4, 0x6c, 0x41, 0, 0x73}, 1)
			}
			if name == "false-page" {
				body := m.Code[0].BodyBytes
				start := bytes.Index(body, []byte{3, 0x40})
				end := bytes.Index(body[start:], []byte{0x0d, 0, 0x0b}) + start + 3
				loop := bytes.Replace(body[start:end], []byte{0x20, 4, 0x6c}, []byte{0x20, 4, 0x6c, 0x41, 0, 0x73}, 1)
				many := append([]byte(nil), body[:start]...)
				for i := 0; i < 4; i++ {
					many = append(many, loop...)
				}
				many = append(many, body[end:len(body)-1]...)
				for len(many) < 511 {
					many = append(many, 1)
				}
				m.Code[0].BodyBytes = append(many, 0x0b)
				if err := wasm.ValidateModule(m); err != nil {
					t.Fatal(err)
				}
			}
			if name == "small" {
				m = benchSmallScalarModule(t)
			}
			if name == "control" {
				m = benchMediumControlModule(t)
			}
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Workers: 1, AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, DeferCodeMapping: deferred, Stats: &stats})
			if err != nil {
				t.Fatal(err)
			}
			if (name == "false-positive" || name == "false-page") && stats.Funcs[0].Peephole["region-loop-map32"] != 0 {
				t.Fatal("false positive emitted SIMD")
			}
			allocs := testing.AllocsPerRun(10, func() {
				x, err := CompileModuleWith(m, CompileOptions{Workers: 1, AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, DeferCodeMapping: deferred})
				if err != nil {
					t.Fatal(err)
				}
				if x.CodeImage != nil {
					x.CodeImage.Close()
				}
			})
			t.Logf("allocations=%g", allocs)
			t.Logf("%s heap=%t native=%d arena=%d executable=%d hints=%d+%d", name, deferred, len(cm.Code), stats.NativeSize.CompilerCodeArenaBytes, stats.NativeSize.ExecutableMappingBytes, stats.Compile.HintHeaderBytes, stats.Compile.HintSidecarBytes)
			if cm.CodeImage != nil {
				cm.CodeImage.Close()
			}
		}
	}
	// Test the serial heap trim with a base estimate above 256 KiB.
	m := directMap32Fixture(t, false, false)
	fn, typ := m.Code[0], m.FuncTypes[0]
	m.Code = make([]wasm.Func, 1024)
	m.FuncTypes = make([]wasm.TypeIdx, 1024)
	for i := range m.Code {
		m.Code[i], m.FuncTypes[i] = fn, typ
	}
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{Workers: 1, AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, DeferCodeMapping: true, Stats: &stats})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("1024-maps heap=true native=%d arena=%d executable=%d hints=%d+%d", len(cm.Code), stats.NativeSize.CompilerCodeArenaBytes, stats.NativeSize.ExecutableMappingBytes, stats.Compile.HintHeaderBytes, stats.Compile.HintSidecarBytes)
	if cm.CodeImage != nil {
		cm.CodeImage.Close()
	}
}
