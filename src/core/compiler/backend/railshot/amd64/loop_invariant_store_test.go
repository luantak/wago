//go:build amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func invariantStoreLoopBody() []byte {
	return []byte{
		0x20, 0, 0x20, 3, 0x20, 1, 0x28, 2, 0, 0x6a, 0x22, 3, 0x36, 2, 0,
		0x20, 1, 0x41, 4, 0x6a, 0x21, 1,
		0x20, 2, 0x41, 1, 0x6a, 0x22, 2, 0x41, 32, 0x47, 0x0d, 0, 0x0b,
	}
}

func TestInvariantStoreLoopRejectsUnsafePlans(t *testing.T) {
	base := invariantStoreLoopBody()
	bad := map[string][]byte{}
	bad["varying-destination"] = append([]byte{}, base...)
	bad["varying-destination"][1] = 1
	bad["trapping-arithmetic"] = append([]byte{}, base...)
	bad["trapping-arithmetic"][9] = 0x6d
	bad["nonlinear-stream"] = append([]byte{}, base...)
	pos := bytes.Index(base, []byte{0x20, 1, 0x41, 4, 0x6a})
	bad["nonlinear-stream"][pos+4] = 0x6c
	bad["changing-limit"] = append([]byte{}, base...)
	pos = bytes.Index(base, []byte{0x41, 32, 0x47})
	bad["changing-limit"][pos], bad["changing-limit"][pos+1] = 0x20, 1
	bad["known-alias"] = append([]byte{}, base...)
	bad["known-alias"][5] = 0
	bad["nested-control"] = append([]byte{0x02, 0x40}, base...)
	bad["call"] = append([]byte{0x10, 0}, base...)
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			f := fn{nLocals: 4, localType: []machineType{mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
			if _, ok := f.inspectInvariantStoreLoop(wasm.NewReader(body)); ok {
				t.Fatal("unsafe loop accepted")
			}
		})
	}
}

func TestInvariantStoreLoopPlan(t *testing.T) {
	f := fn{nLocals: 4, localType: []machineType{mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	r := wasm.NewReader(invariantStoreLoopBody())
	p, ok := f.inspectInvariantStoreLoop(r)
	if !ok {
		t.Fatal("invariant store loop was not planned")
	}
	if r.Offset() != 0 || p.p.storeN != 1 || p.p.loadN != 1 || p.p.induction != 2 || p.p.step != 1 || p.p.loads[0].stride != 4 {
		t.Fatalf("bad plan: %+v", p)
	}
}
