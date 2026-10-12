//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestRegionLoopAffineDependencies(t *testing.T) {
	for _, op := range []byte{0x6a, 0x6b, 0x6c, 0x74} {
		for _, scale := range []uint32{0, 1, 2, 31, 32, 0x80000000, 0xffffffff} {
			var p regionLoopPlan
			local, ok := p.local(0, mtI32)
			if !ok {
				t.Fatal("local rejected")
			}
			x := p.locals[local].initial
			constant := p.add(regionLoopNode{bits: uint64(scale)})
			// Include a coefficient that multiplication can erase modulo 2^32.
			high := p.integer(0x74, x, p.add(regionLoopNode{bits: 31}))
			id := p.integer(op, high, constant)
			if id == 0 {
				t.Fatal("affine operation rejected", op, scale)
			}
			node := p.nodes[id]
			for _, input := range []uint32{0, 1, 2, 0xffffffff} {
				want := input << 31
				switch op {
				case 0x6a:
					want += scale
				case 0x6b:
					want -= scale
				case 0x6c:
					want *= scale
				case 0x74:
					want <<= scale & 31
				}
				got := uint32(node.bits) + node.coeff[local]*input
				if got != want {
					t.Fatalf("op=%x scale=%x input=%x: got=%x want=%x", op, scale, input, got, want)
				}
			}
			var uses uint8
			for i, c := range node.coeff {
				if c != 0 {
					uses |= 1 << i
				}
			}
			if node.uses != uses {
				t.Fatalf("op=%x scale=%x dependencies=%x want=%x", op, scale, node.uses, uses)
			}
			// Exact cancellation must make the expression invariant.
			zero := p.integer(0x6b, x, x)
			if zero == 0 || p.nodes[zero].bits != 0 || p.nodes[zero].uses != 0 || p.nodes[zero].coeff != [regionLoopMaxLocals]uint32{} {
				t.Fatal("cancelled local remains variant")
			}
		}
	}
}

func TestRegionLoopRejectsTruncatedMap(t *testing.T) {
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		ft, _ := m.LocalFuncType(0)
		var types []machineType
		for _, v := range ft.Params {
			types = append(types, mtOf(v))
		}
		types = append(types, mtOf(ft.Params[4]))
		at := bytes.Index(m.Code[0].BodyBytes, []byte{3, 0x40}) + 2
		body := m.Code[0].BodyBytes[at:]
		classifier := wasm.NewModuleInstructionClassifier(m, false)
		var p regionLoopPlan
		full := wasm.ReaderFrom(body)
		if !inspectRegionLoop(full, types, classifier, &p) || !p.qualifyMap32() {
			t.Fatal("complete map rejected")
		}
		end := p.endPC
		for cut := 0; cut < end; cut++ {
			r := wasm.ReaderFrom(body[:cut])
			if inspectRegionLoop(r, types, classifier, &p) {
				t.Fatalf("fp=%v accepted prefix %d/%d", fp, cut, end)
			}
			if r.Offset() != 0 {
				t.Fatal("inspection consumed caller reader")
			}
		}
	}
}

func TestRegionLoopMapHintGatesGrammar(t *testing.T) {
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		ft, _ := m.LocalFuncType(0)
		var types []machineType
		for _, v := range ft.Params {
			types = append(types, mtOf(v))
		}
		types = append(types, mtOf(ft.Params[4]))
		at := bytes.Index(m.Code[0].BodyBytes, []byte{3, 0x40}) + 2
		r := wasm.ReaderFrom(m.Code[0].BodyBytes[at:])
		classifier := wasm.NewModuleInstructionClassifier(m, false)
		var p regionLoopPlan
		if inspectRegionLoopWithMap32(r, types, classifier, &p, false) {
			t.Fatal("unhinted 32-bit loop accepted")
		}
		if !inspectRegionLoopWithMap32(r, types, classifier, &p, true) || !p.qualifyMap32() {
			t.Fatal("hinted map rejected")
		}
	}
}
