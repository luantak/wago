//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math/rand"
	"testing"
)

func TestIntegerCountedPlan(t *testing.T) {
	f := fn{nLocals: 4, localType: []machineType{mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	body := []byte{
		0x20, 3, 0x20, 1, 0x41, 1, 0x41, 2, 0x20, 2, 0x20, 0, 0x46, 0x1b, 0x74, 0x6a, 0x21, 3,
		0x20, 2, 0x41, 3, 0x6b, 0x21, 2,
		0x20, 1, 0x41, 2, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b,
	}
	if _, ok := f.inspectIntegerMemoryLoop(wasm.NewReader(body)); ok {
		t.Fatal("extended plan leaked into existing emitter")
	}
	reader := wasm.NewReader(body)
	p, ok := f.inspectIntegerLoop(reader, true)
	if !ok || reader.Offset() != 0 {
		t.Fatal("general counted loop not planned non-destructively")
	}
	if p.loadN != 0 || p.induction != 1 || p.steps[1] != 2 || p.steps[2] != uint32(0xfffffffd) || p.reductionN != 1 || p.reductions[0].local != 3 {
		t.Fatalf("wrong proof: %+v", p)
	}
	rng := rand.New(rand.NewSource(73))
	for trial := 0; trial < 1000; trial++ {
		locals := [4]uint32{rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32()}
		if trial%2 == 0 {
			locals[2] = locals[0]
		}
		var v [128]uint32
		for i := 0; i < p.n; i++ {
			n := p.nodes[i]
			switch n.op {
			case 0x20:
				v[i] = locals[n.a]
			case 0x41:
				v[i] = n.v
			case 0x6a:
				v[i] = v[n.a] + v[n.b]
			case 0x6b:
				v[i] = v[n.a] - v[n.b]
			case 0x74:
				v[i] = v[n.a] << (v[n.b] & 31)
			case 0x46:
				if v[n.a] == v[n.b] {
					v[i] = 1
				}
			case 0x47:
				if v[n.a] != v[n.b] {
					v[i] = 1
				}
			case 0x1b:
				v[i] = v[n.b]
				if v[n.v] != 0 {
					v[i] = v[n.a]
				}
			default:
				t.Fatalf("unexpected node %x", n.op)
			}
		}
		shift := uint32(2)
		if locals[2] == locals[0] {
			shift = 1
		}
		term := locals[1] << shift
		if v[p.reductions[0].term] != term || v[p.values[3]] != locals[3]+term || v[p.values[2]] != locals[2]-3 || v[p.values[1]] != locals[1]+2 {
			t.Fatal("DAG differs from scalar oracle")
		}
	}
	// Effects and nonlinear loop-carried updates must still be rejected.
	bad := append([]byte(nil), body...)
	bad[15] = 0x6c
	for _, b := range [][]byte{bad, append([]byte{0x20, 0, 0x41, 0, 0x36, 2, 0}, body...)} {
		if _, ok := f.inspectIntegerLoop(wasm.NewReader(b), true); ok {
			t.Fatal("unsafe extended loop accepted")
		}
	}
}
