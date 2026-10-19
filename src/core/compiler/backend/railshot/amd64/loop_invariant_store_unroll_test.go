//go:build amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestInvariantStoreUnrollProofSites(t *testing.T) {
	f := fn{nLocals: 4, localType: []machineType{mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	body := invariantStoreLoopBody()
	p, ok := f.inspectInvariantStoreLoop(wasm.NewReader(body))
	if !ok {
		t.Fatal("loop not planned")
	}
	_, sites := invariantStorePrefixPolicy(body, p, 4, f.classifier, true)
	if len(sites) != 2*invariantStoreUnrollFactor-1 {
		t.Fatalf("unrolled prefix load proofs=%d want%d", len(sites), 2*invariantStoreUnrollFactor-1)
	}
}
