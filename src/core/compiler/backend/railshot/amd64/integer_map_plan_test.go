//go:build amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestIntegerLoadStoreMapPlan(t *testing.T) {
	f := fn{nLocals: 4, localType: []machineType{mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	body := []byte{
		0x20, 2, 0x20, 3, 0x28, 2, 12, 0x41, 7, 0x73, 0x36, 2, 4,
		0x20, 2, 0x41, 4, 0x6a, 0x21, 2,
		0x20, 3, 0x41, 4, 0x6a, 0x21, 3,
		0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b,
	}
	if _, ok := f.inspectIntegerLoopWithStores(wasm.NewReader(body), true, true); ok {
		t.Fatal("existing emitter admitted load/store plan")
	}
	r := wasm.NewReader(body)
	p, ok := f.inspectIntegerMapLoop(r, true, true, true)
	if !ok || r.Offset() != 0 || p.loadN != 1 || p.storeN != 1 || p.reductionN != 0 {
		t.Fatalf("bad plan: ok=%v loads=%d stores=%d", ok, p.loadN, p.storeN)
	}
	if p.loads[0].stride != 4 || p.loads[0].offset != 12 || p.loads[0].address.coeff[3] != 1 || p.stores[0].stride != 4 || p.stores[0].offset != 4 {
		t.Fatal("lost independent pointer stride or widened immediate")
	}
	// An address depending on loaded data is not an affine induction stream.
	dependent := bytes.Replace(body, []byte{0x20, 3, 0x28, 2, 12}, []byte{0x20, 3, 0x28, 2, 0, 0x28, 2, 12}, 1)
	if _, ok := f.inspectIntegerMapLoop(wasm.NewReader(dependent), true, true, true); ok {
		t.Fatal("load-dependent address admitted")
	}
	for _, mode := range [][2]bool{{false, true}, {true, false}, {false, false}} {
		if _, ok := f.inspectIntegerMapLoop(wasm.NewReader(body), mode[0], mode[1], true); ok {
			t.Fatal("load/store proof escaped its mode")
		}
	}
}
