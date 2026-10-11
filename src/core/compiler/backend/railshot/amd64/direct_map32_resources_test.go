//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// Untimed measurement of actual staging/mapping bytes, rather than Go B/op.
func TestDirectMap32CapacityResources(t *testing.T) {
	if !diagnosticsEnabled {
		t.Skip("requires wago_codegenstats")
	}
	for _, name := range []string{"i32", "f32", "rejected", "small", "control"} {
		for _, deferred := range []bool{false, true} {
			m := directMap32Fixture(t, name == "f32", name == "rejected")
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
