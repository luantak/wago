//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math"
	"testing"
)

func integerPacketProgramBody(lanes int) []byte {
	var b []byte
	for i := lanes - 1; i >= 0; i-- {
		b = append(b, 0x20, byte(4+i), 0x20, 1, 0x41, byte(i*4&127))
		if i*4 >= 64 {
			b[len(b)-1] |= 128
			b = append(b, 0)
		}
		b = append(b, 0x6a, 0x28, 2, 0, 0x41, 7, 0x6a, 0x6c)
	}
	for i := 1; i < lanes; i++ {
		b = append(b, 0x6a)
	}
	// Generic scalar post-processing: divide, xor with the old state, multiply.
	b = append(b, 0x41, 8, 0x6d, 0x20, 2, 0x73, 0x41, 13, 0x6c, 0x21, 2,
		0x20, 1, 0x20, 3, 0x6a, 0x21, 1, 0x20, 0, 0x41, 0x7f, 0x6a, 0x22, 0, 0x0d, 0, 0x0b)
	return b
}

func integerPacketProgramModule(t *testing.T, lanes int) *wasm.Module {
	body := []byte{1, byte(lanes + 1), 0x7f}
	for i := 0; i < lanes; i++ {
		body = append(body, 0x41, byte(i+1), 0x21, byte(4+i))
	}
	loop := bytes.Replace(integerPacketProgramBody(lanes), []byte{0x41, 7, 0x6a, 0x6c}, []byte{0x41, 7, 0x6a, 0x6c, 0x22, byte(4 + lanes)}, 1)
	body = append(body, 0x03, 0x40)
	body = append(body, loop...)
	body = append(body, 0x20, 2, 0x20, 1, 0x73, 0x20, 0, 0x73, 0x20, byte(4+lanes), 0x73, 0x0b)
	return modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
}

func TestIntegerPacketProgramExecution(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, lanes := range []int{4, 8, 16, 32} {
		for _, count := range []uint32{1, 2, 5} {
			for _, base := range []uint32{0, 65500, 0xfffffff0} {
				for _, stride := range []uint32{0, 4, 96, 0xffffffe0} {
					var initial []byte
					setup := func(mem []byte) {
						for i := 0; i < len(mem); i += 4 {
							binary.LittleEndian.PutUint32(mem[i:], uint32(i)*0x9e3779b9^0x80000000)
						}
						initial = append([]byte{}, mem...)
					}
					var stats ModuleStats
					saved := integerPacketAssertFast
					integerPacketAssertFast = count >= 2 && base == 0 && stride < 128
					got, mem, err := runMemAmd64WithOptions(t, integerPacketProgramModule(t, lanes), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": true, "integer-packet-input-hoist": true}}, setup, uint64(count), uint64(base)|0xdeadbeef00000000, 17, uint64(stride))
					integerPacketAssertFast = saved
					index, state, temp := base, uint32(17), uint32(0)
					trap := false
					for row := uint32(0); row < count && !trap; row++ {
						sum := uint32(0)
						for lane := lanes - 1; lane >= 0; lane-- {
							a := uint64(index + uint32(lane*4))
							if a+4 > 65536 {
								trap = true
								break
							}
							v := (binary.LittleEndian.Uint32(initial[a:]) + 7) * uint32(lane+1)
							sum += v
							if lane == lanes-1 {
								temp = v
							}
						}
						state = (uint32(int32(sum)/8) ^ state) * 13
						index += stride
					}
					if (err != nil) != trap || !trap && got != uint64(state^index^temp) || !bytes.Equal(mem, initial) {
						t.Fatalf("lanes=%d count=%d base=%x stride=%x got=%x want=%x err=%v", lanes, count, base, stride, got, state^index^temp, err)
					}
					if stats.Funcs[0].Peephole["integer-packet-vector"] == 0 {
						t.Fatal("packet program was not emitted")
					}
				}
			}
		}
	}
}

func TestIntegerPacketProgramPlan(t *testing.T) {
	saved := integerPacketProgramEnabled
	integerPacketProgramEnabled = true
	defer func() { integerPacketProgramEnabled = saved }()
	for _, lanes := range []int{4, 8, 16, 32} {
		types := make([]machineType, 4+lanes)
		for i := range types {
			types[i] = mtI32
		}
		f := fn{policy: currentCodegenPolicy(), nLocals: len(types), localType: types, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
		p, ok := f.inspectIntegerPacket(wasm.NewReader(integerPacketProgramBody(lanes)))
		if !ok || p.lanes != lanes {
			t.Fatalf("lanes=%d packet program rejected", lanes)
		}
	}
}

func TestIntegerPacketProgramRejectsUnsafe(t *testing.T) {
	saved := integerPacketProgramEnabled
	integerPacketProgramEnabled = true
	defer func() { integerPacketProgramEnabled = saved }()
	good := integerPacketProgramBody(4)
	cases := map[string][]byte{
		"zero-divisor":         bytes.Replace(good, []byte{0x41, 8, 0x6d}, []byte{0x41, 0, 0x6d}, 1),
		"overflowing-divisor":  bytes.Replace(good, []byte{0x41, 8, 0x6d}, []byte{0x41, 0x7f, 0x6d}, 1),
		"unsupported-divisor":  bytes.Replace(good, []byte{0x41, 8, 0x6d}, []byte{0x41, 3, 0x6d}, 1),
		"changing-coefficient": append([]byte{0x20, 4, 0x41, 1, 0x6a, 0x21, 4}, good...),
		"unused-load":          append([]byte{0x20, 1, 0x28, 2, 0, 0x21, 8}, good...),
		"side-effect":          append([]byte{0x20, 1, 0x41, 0, 0x36, 2, 0}, good...),
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			f := fn{policy: currentCodegenPolicy(), nLocals: 9, localType: []machineType{mtI32, mtI32, mtI32, mtI32, mtI32, mtI32, mtI32, mtI32, mtI32}, classifier: wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)}
			if _, ok := f.inspectIntegerPacket(wasm.NewReader(b)); ok {
				t.Fatal("unsafe packet program admitted")
			}
		})
	}
}

func packetI32Constant(v int32) []byte {
	b := []byte{0x41}
	for {
		c := byte(v & 127)
		v >>= 7
		if v == 0 && c&64 == 0 || v == -1 && c&64 != 0 {
			return append(b, c)
		}
		b = append(b, c|128)
	}
}

func TestIntegerPacketProgramDivisionExtremes(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, op := range []byte{0x6d, 0x6e} {
		for _, d := range []uint32{1, 2, 8, 1 << 30, 1 << 31} {
			if op == 0x6d && d == 1<<31 {
				continue
			}
			for _, input := range []uint32{0, 1, 0xffffffff, 0x80000000, 0x80000001, 0x7fffffff} {
				m := integerPacketProgramModule(t, 4)
				m.Code[0].BodyBytes = bytes.Replace(m.Code[0].BodyBytes, []byte{0x41, 8, 0x6d}, append(packetI32Constant(int32(d)), op), 1)
				setup := func(mem []byte) { binary.LittleEndian.PutUint32(mem, input-70) }
				saved := integerPacketAssertFast
				integerPacketAssertFast = true
				var stats ModuleStats
				got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": true}}, setup, 2, 0, 17, 0)
				integerPacketAssertFast = saved
				q := input / d
				if op == 0x6d {
					q = uint32(int32(input) / int32(d))
				}
				want := uint32(17)
				for row := 0; row < 2; row++ {
					want = (q ^ want) * 13
				}
				want ^= 28
				if err != nil || got != uint64(want) || stats.Funcs[0].Peephole["integer-packet-vector"] == 0 {
					t.Fatalf("op=%x d=%x input=%x got=%x want=%x err=%v", op, d, input, got, want, err)
				}
			}
		}
	}
}

func TestIntegerPacketProgramAcrossCalls(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := integerPacketProgramModule(t, 32)
	callee := append(bytes.Repeat([]byte{0x01}, 4096), 0x41, 7, 0x0b)
	m.Code = append(m.Code, wasm.Func{BodyBytes: callee})
	m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
	var call []byte
	for i := 0; i < 4; i++ {
		call = append(call, 0x41, 0)
	}
	call = append(call, 0x10, 1, 0x1a)
	body := append(append([]byte{}, call...), m.Code[0].BodyBytes[:len(m.Code[0].BodyBytes)-1]...)
	body = append(body, call...)
	m.Code[0].BodyBytes = append(body, 0x0b)
	setup := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*97 + 3)
		}
	}
	for _, count := range []uint64{1, 2, 5} {
		want, expected, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": false}}, setup, count, 0, 17, 4)
		if err != nil {
			t.Fatal(err)
		}
		var stats ModuleStats
		saved := integerPacketAssertFast
		integerPacketAssertFast = count >= 2
		got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": true}}, setup, count, 0, 17, 4)
		integerPacketAssertFast = saved
		if err != nil || got != want || !bytes.Equal(mem, expected) || stats.Funcs[0].Peephole["integer-packet-vector"] == 0 || stats.Funcs[0].Calls["regabi"] == 0 {
			t.Fatalf("count=%d got=%x want=%x err=%v stats=%v", count, got, want, err, stats.Funcs[0])
		}
	}
}

func TestIntegerPacketInputHoist(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := integerPacketProgramModule(t, 32)
	setup := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*97 + 3)
		}
	}
	want, expected, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": false}}, setup, 5, 0, 17, 4)
	if err != nil {
		t.Fatal(err)
	}
	var stats ModuleStats
	got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": true, "integer-packet-input-hoist": true}}, setup, 5, 0, 17, 4)
	if err != nil || got != want || !bytes.Equal(mem, expected) {
		t.Fatalf("got=%x want=%x err=%v", got, want, err)
	}
	if stats.Funcs[0].Peephole["integer-packet-input-hoist"] != 8 {
		t.Fatalf("invariant packets not hoisted: %v", stats.Funcs[0].Peephole)
	}
}

func TestIntegerPacketInputHoistPreservesCallerFloats(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := integerPacketProgramModule(t, 32)
	packet := m.Code[0]
	var body []byte
	for i := 0; i < 32; i++ {
		body = append(body, 0x44)
		body = binary.LittleEndian.AppendUint64(body, math.Float64bits(float64(i+1)))
		body = append(body, 0x21, byte(4+i))
	}
	for i := 0; i < 4; i++ {
		body = append(body, 0x20, byte(i))
	}
	body = append(body, 0x10, 1, 0x21, 36)
	body = append(body, 0x20, 4)
	for i := 1; i < 32; i++ {
		body = append(body, 0x20, byte(4+i), 0xa0)
	}
	body = append(body, 0xaa, 0x20, 36, 0x73, 0x0b)
	m.Code = []wasm.Func{{Locals: wasm.Locals{Runs: []wasm.LocalRun{{Count: 32, Type: wasm.F64}, {Count: 1, Type: wasm.I32}}}, BodyBytes: body}, packet}
	m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
	setup := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*97 + 3)
		}
	}
	want, expected, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": false}}, setup, 5, 0, 17, 4)
	if err != nil {
		t.Fatal(err)
	}
	var stats ModuleStats
	saved := integerPacketAssertFast
	integerPacketAssertFast = true
	got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": true, "integer-packet-input-hoist": true}}, setup, 5, 0, 17, 4)
	integerPacketAssertFast = saved
	if err != nil || got != want || !bytes.Equal(mem, expected) || stats.Funcs[0].Calls["regabi"] == 0 || stats.Funcs[1].Peephole["integer-packet-input-hoist"] != 8 {
		t.Fatalf("got=%x want=%x err=%v caller=%v callee=%v", got, want, err, stats.Funcs[0], stats.Funcs[1])
	}
}

func TestIntegerPacketInputHoistRegisterBudget(t *testing.T) {
	requireCompilerDiagnostics(t)
	// Two lane-varying inputs need more vector registers than are available if
	// both are fully cached. Retain one input and pack the other in the loop.
	loop := integerPacketProgramBody(32)
	for i := 31; i >= 0; i-- {
		loop = bytes.Replace(loop, []byte{0x41, 7, 0x6a, 0x6c}, []byte{0x41, byte(i + 1), 0x6a, 0x6c}, 1)
	}
	body := []byte{1, 32, 0x7f}
	for i := 0; i < 32; i++ {
		body = append(body, 0x41, byte(i+1), 0x21, byte(4+i))
	}
	body = append(body, 0x03, 0x40)
	body = append(body, loop...)
	body = append(body, 0x20, 2, 0x0b)
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	setup := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*31 + 7)
		}
	}
	want, expected, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": false}}, setup, 5, 0, 17, 4)
	if err != nil {
		t.Fatal(err)
	}
	var stats ModuleStats
	saved := integerPacketAssertFast
	integerPacketAssertFast = true
	got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true, "integer-packet-program": true, "integer-packet-input-hoist": true}}, setup, 5, 0, 17, 4)
	integerPacketAssertFast = saved
	if err != nil || got != want || !bytes.Equal(mem, expected) || stats.Funcs[0].Peephole["integer-packet-input-hoist"] != 8 {
		t.Fatalf("got=%x want=%x err=%v stats=%v", got, want, err, stats.Funcs[0])
	}
}
