//go:build (linux || darwin || windows) && amd64 && !wago_precompiled

package wago

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
)

func TestDirectMap32AutomaticCPUSelection(t *testing.T) {
	host, ok := cachedAMD64CPUFeatures()
	if !ok {
		t.Fatal("CPU capability detection failed")
	}
	features := selectedAMD64CompileFeatures(host)
	if !hostSupportsBMI2() {
		features &^= shared.AMD64BMI2
	}
	for _, fp := range []bool{false, true} {
		typ, load, mul, add, store := wasm.I32, byte(0x28), byte(0x6c), byte(0x6a), byte(0x36)
		if fp {
			typ, load, mul, add, store = wasm.F32, 0x2a, 0x94, 0x92, 0x38
		}
		local := byte(0x7f)
		if fp {
			local = 0x7d
		}
		body := []byte{1, 1, local, 0x02, 0x40, 0x20, 3, 0x45, 0x0d, 0, 0x03, 0x40,
			0x20, 0, 0x20, 1, load, 0, 0, 0x20, 4, mul, 0x20, 5, add, 0x22, 6, store, 0, 0,
			0x20, 0, 0x41, 4, 0x6a, 0x21, 0, 0x20, 1, 0x41, 4, 0x6a, 0x21, 1,
			0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x20, 3, 0x47, 0x0d, 0, 0x0b, 0x0b, 0x20, 6}
		if fp {
			body = append(body, 0xbc)
		}
		body = append(body, 0x0b)
		raw := wasmtest.Module(
			wasmtest.Section(1, wasmtest.Vec(wasmtest.FuncType([]wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, typ, typ}, []wasm.ValType{wasm.I32}))),
			wasmtest.Section(3, wasmtest.Vec(wasmtest.ULEB(0))),
			wasmtest.Section(5, wasmtest.Vec([]byte{0, 1})),
			wasmtest.Section(10, wasmtest.Vec(append(wasmtest.ULEB(uint32(len(body))), body...))),
		)
		m, err := wasm.DecodeModule(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := wasm.ValidateModule(m); err != nil {
			t.Fatal(err)
		}
		var stats railshotModuleStats
		opts := railshotCompileOptions{Workers: 1, DeferCodeMapping: true}
		if compilerTelemetryEnabled {
			opts.Stats = &stats
		}
		automatic, err := railshotCompileModuleWith(m, opts)
		if err != nil {
			t.Fatal(err)
		}
		if compilerTelemetryEnabled && stats.Funcs[0].Peephole["region-loop-map32"] == 0 {
			t.Fatalf("fp=%v features=%x did not select SIMD map", fp, features)
		}
		opts.AMD64FeaturesSet = true
		opts.AMD64Features = features
		explicit, err := railshotCompileModuleWith(m, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(automatic.Code, explicit.Code) {
			t.Fatalf("fp=%v automatic code differs from detected profile %x", fp, features)
		}
		if automatic.RequiredAMD64Features&^uint32(features) != 0 {
			t.Fatalf("fp=%v requires unsupported features %x", fp, automatic.RequiredAMD64Features)
		}
		wantAVX2 := !fp && features.Has(shared.AMD64SSE41|shared.AMD64AVX|shared.AMD64AVX2)
		if automatic.RequiresAVX2 != wantAVX2 {
			t.Fatalf("fp=%v AVX2=%v want=%v", fp, automatic.RequiresAVX2, wantAVX2)
		}
	}
}
