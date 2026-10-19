//go:build linux && amd64

package amd64

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// Two independent memory streams feed an integer expression DAG. The old
// accumulator can appear on either side of nested additions.
func integerMemoryLoopBody() []byte {
	return []byte{
		0x20, 0, 0x20, 3, 0x6a, 0x28, 2, 0, 0x41, 3, 0x73,
		0x20, 4, 0x20, 1, 0x20, 3, 0x6a, 0x28, 2, 4, 0x41, 7, 0x6a,
		0x6a, 0x6a, 0x21, 4,
		0x20, 3, 0x41, 4, 0x6a, 0x22, 3, 0x20, 2, 0x47, 0x0d, 0, 0x0b,
	}
}

func TestIntegerMemoryLoopAdmission(t *testing.T) {
	f := fn{nLocals: 5, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	r := wasm.NewReader(integerMemoryLoopBody())
	_, ok := f.inspectIntegerMemoryLoop(r)
	if !ok {
		t.Fatal("general integer memory reduction was not planned")
	}
	if r.Offset() != 0 {
		t.Fatal("lookahead consumed the scalar reader")
	}
}

func TestIntegerMemoryLoopRejectsUnsafeGraphs(t *testing.T) {
	original := integerMemoryLoopBody()
	cases := map[string][]byte{}
	cases["effect"] = append([]byte{0x20, 0, 0x41, 0, 0x36, 2, 0}, original...)
	cases["data-address"] = append(append(append([]byte{}, original[:8]...), 0x28, 2, 0), original[8:]...)
	cases["carried-address"] = append([]byte{}, original...)
	cases["carried-address"][1] = 4
	cases["nonlinear-accumulator"] = append([]byte{}, original...)
	pos := bytes.Index(original, []byte{0x6a, 0x6a, 0x21, 4})
	cases["nonlinear-accumulator"][pos] = 0x6c
	cases["duplicate-accumulator"] = append([]byte{0x20, 4}, original[11:]...)
	cases["changing-limit"] = append([]byte{}, original...)
	cases["changing-limit"][len(original)-5] = 3
	cases["non-power-step"] = append([]byte{}, original...)
	pos = bytes.Index(original, []byte{0x20, 3, 0x41, 4, 0x6a, 0x22, 3})
	cases["non-power-step"][pos+3] = 3
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			f := fn{nLocals: 5, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
			r := wasm.NewReader(code)
			if _, ok := f.inspectIntegerMemoryLoop(r); ok {
				t.Fatal("unsafe graph admitted")
			}
			if r.Offset() != 0 {
				t.Fatal("rejection consumed scalar reader")
			}
		})
	}
	for end := 0; end < len(original); end++ {
		f := fn{nLocals: 5, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		if _, ok := f.inspectIntegerMemoryLoop(wasm.NewReader(original[:end])); ok {
			t.Fatalf("truncated body %d admitted", end)
		}
	}
}

func TestIntegerMemoryAffineAndReductionOracle(t *testing.T) {
	f := fn{nLocals: 5, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
	p, ok := f.inspectIntegerMemoryLoop(wasm.NewReader(integerMemoryLoopBody()))
	if !ok {
		t.Fatal("graph rejected")
	}
	if p.induction != 3 || p.step != 4 || p.loadN != 2 || p.reductionN != 1 {
		t.Fatalf("wrong loop plan: %+v", p)
	}
	rng := rand.New(rand.NewSource(19))
	load := func(addr uint64) uint32 { return uint32(addr*0x9e3779b97f4a7c15) ^ uint32(addr>>32) }
	for trial := 0; trial < 1000; trial++ {
		var locals [32]uint32
		for i := range locals {
			locals[i] = rng.Uint32()
		}
		var values [128]uint32
		for i := 0; i < p.n; i++ {
			n := p.nodes[i]
			switch n.op {
			case 0x20:
				values[i] = locals[n.a]
			case 0x41:
				values[i] = n.v
			case 0x6a:
				values[i] = values[n.a] + values[n.b]
			case 0x73:
				values[i] = values[n.a] ^ values[n.b]
			case 0x28:
				for _, stream := range p.loads[:p.loadN] {
					if stream.node != i {
						continue
					}
					addr := stream.address.constant
					for j, c := range stream.address.coeff {
						addr += c * locals[j]
					}
					if addr != values[n.a] {
						t.Fatal("affine address changed modular arithmetic")
					}
					values[i] = load(uint64(addr) + stream.offset)
				}
			}
		}
		a := load(uint64(locals[0]+locals[3])) ^ 3
		b := load(uint64(locals[1]+locals[3])+4) + 7
		want := locals[4] + a + b
		red := p.reductions[0]
		if values[p.values[4]] != want || locals[red.local]+values[red.term] != want {
			t.Fatalf("reassociated reduction changed result at trial %d", trial)
		}
	}
}
