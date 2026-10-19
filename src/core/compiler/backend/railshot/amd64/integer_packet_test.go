//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestIntegerPacketRejectsUnsafe(t *testing.T) {
	good := integerPacketBody(4)
	cases := map[string][]byte{
		"constant":        bytes.Replace(good, []byte{0x41, 7, 0x73}, []byte{0x41, 8, 0x73}, 1),
		"operator":        bytes.Replace(good, []byte{0x41, 3, 0x6c}, []byte{0x41, 3, 0x6b}, 1),
		"noncontiguous":   bytes.Replace(good, []byte{0x41, 12, 0x6a, 0x28}, []byte{0x41, 16, 0x6a, 0x28}, 1),
		"unused-load":     append([]byte{0x20, 0, 0x28, 2, 0, 0x21, 6}, good...),
		"call":            append([]byte{0x10, 0}, good...),
		"store":           append([]byte{0x20, 0, 0x41, 0, 0x36, 2, 0}, good...),
		"extra-carry":     append([]byte{0x20, 6, 0x41, 1, 0x6a, 0x21, 6}, good...),
		"changing-stride": append([]byte{0x20, 5, 0x41, 1, 0x6a, 0x21, 5}, good...),
		"changing-base":   append([]byte{0x20, 0, 0x41, 1, 0x6a, 0x21, 0}, good...),
		"wrong-countdown": bytes.Replace(good, []byte{0x41, 0x7f, 0x6a}, []byte{0x41, 0x7e, 0x6a}, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			f := fn{nLocals: 7, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
			if _, ok := f.inspectIntegerPacket(wasm.NewReader(body)); ok {
				t.Fatal("unsafe packet admitted")
			}
		})
	}
}

// An ordinary polynomial, not a particular reduction kernel. Each packet has
// independent contiguous inputs and a uniform scalar expression per lane.
func integerPacketBody(lanes int) []byte {
	var body []byte
	for lane := lanes - 1; lane >= 0; lane-- {
		for _, base := range []byte{0, 1} {
			body = append(body, 0x20, base, 0x20, 3, 0x6a, 0x41, byte(lane*4), 0x6a, 0x28, 2, 0)
		}
		body = append(body, 0x6b, 0x41, 3, 0x6c, 0x41, 7, 0x73)
	}
	body = append(body, 0x20, 4)
	for i := 0; i < lanes; i++ {
		body = append(body, 0x6a)
	}
	body = append(body, 0x21, 4, 0x20, 3, 0x20, 5, 0x6a, 0x21, 3, 0x20, 2, 0x41, 0x7f, 0x6a, 0x22, 2, 0x0d, 0, 0x0b)
	return body
}

func TestIntegerPacketPlan(t *testing.T) {
	for _, lanes := range []int{4, 8} {
		f := fn{nLocals: 6, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		r := wasm.NewReader(integerPacketBody(lanes))
		p, ok := f.inspectIntegerPacket(r)
		if !ok || p.lanes != lanes || p.counter != 2 || p.index != 3 || p.acc != 4 {
			t.Fatalf("lanes=%d plan=%+v ok=%v", lanes, p, ok)
		}
		if r.Offset() != 0 {
			t.Fatal("lookahead consumed scalar reader")
		}
	}
}
