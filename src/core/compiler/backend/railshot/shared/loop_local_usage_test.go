package shared

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestLoopLocalUsageProof(t *testing.T) {
	classifier := wasm.NewModuleInstructionClassifier(nil, false)
	body := []byte{0x20, 1, 0x20, 1, 0x7c, 0x22, 1, 0x1a, 0x20, 0, 0x41, 1, 0x6b, 0x22, 0, 0x0d, 0, 0x0b}
	r := wasm.NewReader(body)
	reads, writes, written, ok := LoopLocalUsage(r, classifier, 2)
	if !ok || reads[1] != 2 || reads[0] != 1 || writes[0] != 1 || writes[1] != 1 || written != 3 || r.Offset() != 0 {
		t.Fatalf("reads=%v writes=%v written=%x ok=%v offset=%d", reads, writes, written, ok, r.Offset())
	}
	for cut := 0; cut < len(body); cut++ {
		if _, _, _, ok := LoopLocalUsage(wasm.NewReader(body[:cut]), classifier, 2); ok {
			t.Fatalf("accepted prefix %d", cut)
		}
	}
	for _, invalid := range [][]byte{
		{0x10, 0},        // call
		{0x03, 0x40},     // nested loop
		{0x04, 0x40},     // another exit
		{0x0d, 1},        // forward exit
		{0x0c, 0},        // unconditional backedge
		{0x20, 2},        // invalid local
		{0xfc, 10, 0, 0}, // bulk helper
		{0xfd, 12},       // SIMD needs a separate register contract
		{0x0d, 0, 0x01},  // work after the backedge
		bytes.Repeat([]byte{1}, 96),
	} {
		bad := append(append([]byte{}, invalid...), body...)
		if _, _, _, ok := LoopLocalUsage(wasm.NewReader(bad), classifier, 2); ok {
			t.Fatalf("accepted %x", invalid)
		}
	}
}
