package wago

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
)

func TestPreparedScalarCombinedAdmission(t *testing.T) {
	saved := preparedScalarCombinedEnabled
	preparedScalarCombinedEnabled = true
	defer func() { preparedScalarCombinedEnabled = saved }()
	m := wasmtest.Module(
		wasmtest.Section(1, wasmtest.Vec(wasmtest.FuncType([]wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}))),
		wasmtest.Section(3, wasmtest.Vec(wasmtest.ULEB(0))),
		wasmtest.Section(7, wasmtest.Vec(wasmtest.ExportEntry("f", 0, 0))),
		wasmtest.Section(10, wasmtest.Vec(wasmtest.Code([]byte{0x41, 42, 0x20, 0, 0x6d, 0x0b}))),
	)
	c := MustCompile(m)
	defer c.Close()
	in, err := Instantiate(c, InstantiateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	fn, err := in.WasmFunc("f")
	if err != nil {
		t.Fatal(err)
	}
	// Exercise private scalar routing rather than the independent register ABI.
	fn.directIntFast, fn.isolatedFast, fn.privateFast = false, false, true
	// The memory-free fixture initially prepares isolated entry; remove that
	// unused cached state to exercise allocation on the first private call too.
	fn.directGate = nil
	in.pluginState.Store(nil)
	var state *instancePluginState
	for _, x := range []uint64{2, 0, 3} {
		out, err := fn.Invoke(x)
		state = in.pluginState.Load()
		if state == nil {
			t.Fatal("first scalar call did not prepare admission state")
		}
		if x == 0 {
			if err == nil {
				t.Fatal("division trap lost")
			}
		} else if err != nil || len(out) != 1 || out[0] != 42/x {
			t.Fatalf("x=%d out=%v err=%v", x, out, err)
		}
		if in.invocationState.word.Load() != 0 || state.invocationID != 0 {
			t.Fatal("scalar entry retained ownership")
		}
	}
	state.invokeMu.revokeFast()
	out, err := fn.Invoke(2)
	if err != nil || len(out) != 1 || out[0] != 21 {
		t.Fatalf("revoked fallback: %v %v", out, err)
	}
	if in.invocationState.Load() != 0 || state.invokeMu.state.Load() != invocationGateRevoked {
		t.Fatal("revoked fallback lost gate state")
	}
	if err := in.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := fn.Invoke(2); err == nil {
		t.Fatal("closed scalar handle admitted")
	}
}
