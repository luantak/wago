//go:build amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math"
	"testing"
)

func TestFloatMapPlanPreservesExpressionAndCounter(t *testing.T) {
	f := fn{m: &wasm.Module{}, nLocals: 3, localType: []machineType{mtI32, mtI32, mtI32}}
	for _, wide := range []bool{false, true} {
		load, store, constant, mul, sub, step := byte(0x2a), byte(0x38), byte(0x43), byte(0x94), byte(0x93), byte(4)
		if wide {
			load, store, constant, mul, sub, step = 0x2b, 0x39, 0x44, 0xa2, 0xa1, 8
		}
		body := []byte{0x20, 1, 0x20, 2, 0x6a, 0x20, 0, 0x20, 2, 0x6a, load, 0, 0, constant}
		if wide {
			body = binary.LittleEndian.AppendUint64(body, math.Float64bits(1.125))
		} else {
			body = binary.LittleEndian.AppendUint32(body, math.Float32bits(1.125))
		}
		body = append(body, mul, constant)
		if wide {
			body = binary.LittleEndian.AppendUint64(body, math.Float64bits(0.25))
		} else {
			body = binary.LittleEndian.AppendUint32(body, math.Float32bits(0.25))
		}
		body = append(body, sub, store, 0, 0, 0x20, 2, 0x41, step, 0x6a, 0x22, 2, 0x41, 0x80, 1, 0x47, 0x0d, 0, 0x0b)
		r := wasm.NewReader(body)
		p, ok := f.inspectFloatMapLoop(r)
		if !ok || r.Offset() != 0 || p.counter != 2 || p.limit != 128 || p.step != uint32(step) || p.wide != wide {
			t.Fatalf("bad plan wide=%v ok=%v counter=%d limit=%d step=%d", wide, ok, p.counter, p.limit, p.step)
		}
		root := p.nodes[p.result]
		if root.op != sub || p.nodes[root.a].op != mul || p.nodes[root.b].op != constant {
			t.Fatal("expression tree reordered")
		}
		if p.addresses[p.dest].base != 1 || p.addresses[p.dest].invariant {
			t.Fatal("bad destination address")
		}
		wrongStride := bytes.Replace(body, []byte{0x41, step, 0x6a, 0x22, 2}, []byte{0x41, 3, 0x6a, 0x22, 2}, 1)
		if _, ok := f.inspectFloatMapLoop(wasm.NewReader(wrongStride)); ok {
			t.Fatal("unsupported stride accepted")
		}
		withCall := append([]byte{0x10, 0}, body...)
		if _, ok := f.inspectFloatMapLoop(wasm.NewReader(withCall)); ok {
			t.Fatal("call accepted")
		}
		withCarry := append([]byte{0x20, 0, 0x41, 1, 0x6a, 0x21, 0}, body...)
		if _, ok := f.inspectFloatMapLoop(wasm.NewReader(withCarry)); ok {
			t.Fatal("loop carried source pointer accepted")
		}
		mixed := append([]byte{}, body...)
		pos := bytes.Index(mixed, []byte{store, 0, 0, 0x20, 2})
		mixed[pos] = store ^ 1
		if _, ok := f.inspectFloatMapLoop(wasm.NewReader(mixed)); ok {
			t.Fatal("mixed float widths accepted")
		}
	}
}
