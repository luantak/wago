//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math/rand"
	"testing"
)

func TestIntegerStorePlan(t *testing.T) {
	f := fn{nLocals: 4, localType: []machineType{mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	body := []byte{
		0x20, 2, 0x20, 1, 0x20, 1, 0x6c, 0x41, 7, 0x73, 0x36, 2, 0,
		0x20, 2, 0x20, 3, 0x20, 1, 0x6a, 0x36, 2, 4,
		0x20, 2, 0x41, 8, 0x6a, 0x21, 2,
		0x20, 1, 0x41, 1, 0x6a, 0x22, 1, 0x20, 0, 0x47, 0x0d, 0, 0x0b,
	}
	if _, ok := f.inspectIntegerLoop(wasm.NewReader(body), true); ok {
		t.Fatal("stores admitted by reduction emitter")
	}
	r := wasm.NewReader(body)
	p, ok := f.inspectIntegerLoopWithStores(r, true, true)
	if !ok || r.Offset() != 0 || p.storeN != 2 || p.reductionN != 0 || p.stores[0].stride != 8 || p.stores[1].stride != 8 {
		t.Fatal("independent store map not proved")
	}
	if !integerStoreDisjointResidues(p.stores[0], p.stores[1]) {
		t.Fatal("interleaved stores not proved disjoint")
	}
	var q pureReducePlan
	q.nodes, q.values, q.written, q.n, q.locals, q.counter, q.induction = p.nodes, p.values, p.written, p.n, p.locals, p.induction, p.induction
	q.compactScratch = false
	q.outputN = p.storeN
	for i := 0; i < p.storeN; i++ {
		q.outputRoots[i] = p.stores[i].value
	}
	if _, ok := f.planPureVectors(&q); !ok {
		t.Fatal("store results did not fit vector layout")
	}
	rng := rand.New(rand.NewSource(79))
	for trial := 0; trial < 1000; trial++ {
		locals := [4]uint32{rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32()}
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
			case 0x6c:
				v[i] = v[n.a] * v[n.b]
			case 0x73:
				v[i] = v[n.a] ^ v[n.b]
			case 0x47:
				if v[n.a] != v[n.b] {
					v[i] = 1
				}
			default:
				t.Fatalf("unexpected op %x", n.op)
			}
		}
		if v[p.stores[0].value] != (locals[1]*locals[1])^7 || v[p.stores[1].value] != locals[3]+locals[1] {
			t.Fatal("stored values differ from scalar oracle")
		}
		for _, s := range p.stores[:p.storeN] {
			addr := s.address.constant
			for i, c := range s.address.coeff {
				if i < len(locals) {
					addr += c * locals[i]
				}
			}
			if addr != locals[2] {
				t.Fatal("store affine address differs")
			}
		}
	}
	loadDependent := append([]byte{0x20, 2, 0x28, 2, 0, 0x21, 3}, body...)
	carried := append([]byte{0x20, 3, 0x20, 1, 0x6a, 0x21, 3}, body...)
	for _, code := range [][]byte{loadDependent, carried} {
		if _, ok := f.inspectIntegerLoopWithStores(wasm.NewReader(code), true, true); ok {
			t.Fatal("dependent store map accepted")
		}
	}
}

func TestIntegerStoreDisjointResidues(t *testing.T) {
	for _, stride := range []uint32{4, 8, 12, 16, 32} {
		for offset := uint64(0); offset < 64; offset++ {
			a, b := integerMemoryStore{stride: stride}, integerMemoryStore{stride: stride, offset: offset}
			if integerStoreDisjointResidues(a, b) {
				for i := uint64(0); i < 32; i++ {
					for j := uint64(0); j < 32; j++ {
						x, y := i*uint64(stride), offset+j*uint64(stride)
						if x < y+4 && y < x+4 {
							t.Fatal("overlapping store intervals accepted")
						}
					}
				}
			}
			b.address.coeff[0] = 1
			if integerStoreDisjointResidues(a, b) {
				t.Fatal("unrelated bases proved disjoint")
			}
		}
	}
	rng := rand.New(rand.NewSource(83))
	for trial := 0; trial < 2000; trial++ {
		stride := uint32(1) << uint(rng.Intn(29)+2)
		a := integerMemoryStore{stride: stride, offset: uint64(rng.Uint32()), address: integerMemoryAddress{constant: rng.Uint32()}}
		b := integerMemoryStore{stride: stride, offset: uint64(rng.Uint32()), address: integerMemoryAddress{constant: rng.Uint32()}}
		if !integerStoreDisjointResidues(a, b) {
			continue
		}
		base := rng.Uint32()
		for i := uint32(0); i < 16; i++ {
			for j := uint32(0); j < 16; j++ {
				x := uint64(base+a.address.constant+i*stride) + a.offset
				y := uint64(base+b.address.constant+j*stride) + b.offset
				if x < y+4 && y < x+4 {
					t.Fatal("modular address residues overlap")
				}
			}
		}
	}
}
