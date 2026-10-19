//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func integerPacketModule(t *testing.T, lanes int) *wasm.Module {
	loop := integerPacketBody(lanes)
	// Keep an overwritten temporary observable after the loop. The original
	// final scalar iteration, not a reconstruction shortcut, must define it.
	loop = bytes.Replace(loop, []byte{0x41, 7, 0x73}, []byte{0x41, 7, 0x73, 0x22, 6}, 1)
	body := append([]byte{1, 1, 0x7f, 0x03, 0x40}, loop...)
	body = append(body, 0x20, 4, 0x20, 3, 0x73, 0x20, 2, 0x73, 0x20, 6, 0x73, 0x0b)
	return modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
}

func TestIntegerPacketExecution(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, lanes := range []int{4, 8} {
		for _, count := range []uint32{1, 2, 3, 8} {
			for _, stride := range []uint32{0, 4, 48, 0xfffffff0} {
				for _, base := range []uint32{0, 65500, 0xfffffffc} {
					var initial []byte
					setup := func(mem []byte) {
						for i := 0; i < len(mem); i += 4 {
							binary.LittleEndian.PutUint32(mem[i:], uint32(i)*0x9e3779b9^0x80000000)
						}
						initial = append([]byte{}, mem...)
					}
					var stats ModuleStats
					saved := integerPacketAssertFast
					integerPacketAssertFast = count >= 2 && base == 0 && stride < 64
					got, mem, err := runMemAmd64WithOptions(t, integerPacketModule(t, lanes), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true}}, setup, uint64(base)|0xdeadbeef00000000, 512, uint64(count), 0, 17, uint64(stride))
					integerPacketAssertFast = saved
					index, sum, temp := uint32(0), uint32(17), uint32(0)
					trap := false
					for row := uint32(0); row < count && !trap; row++ {
						for lane := lanes - 1; lane >= 0; lane-- {
							a, b := uint64(base+index+uint32(lane*4)), uint64(512+index+uint32(lane*4))
							if a+4 > 65536 || b+4 > 65536 {
								trap = true
								break
							}
							v := ((binary.LittleEndian.Uint32(initial[a:]) - binary.LittleEndian.Uint32(initial[b:])) * 3) ^ 7
							sum += v
							if lane == lanes-1 {
								temp = v
							}
						}
						index += stride
					}
					want := sum ^ index ^ temp
					if (err != nil) != trap || !trap && got != uint64(want) || !bytes.Equal(mem, initial) {
						t.Fatalf("lanes=%d count=%d stride=%x base=%x got=%x want=%x err=%v", lanes, count, stride, base, got, want, err)
					}
					if stats.Funcs[0].Peephole["integer-packet-vector"] == 0 {
						t.Fatal("packet prefix not emitted")
					}
				}
			}
		}
	}
}

func TestIntegerPacketSignedExpression(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, lanes := range []int{4, 8} {
		for _, count := range []uint32{1, 2, 5} {
			for _, base := range []uint32{0, 65500} {
				loop := bytes.ReplaceAll(integerPacketBody(lanes), []byte{0x6b, 0x41, 3, 0x6c, 0x41, 7, 0x73}, []byte{0x6b, 0x22, 6, 0x20, 6, 0x41, 31, 0x75, 0x22, 7, 0x73, 0x20, 7, 0x6b})
				body := append([]byte{1, 2, 0x7f, 0x03, 0x40}, loop...)
				body = append(body, 0x20, 4, 0x20, 3, 0x73, 0x20, 2, 0x73, 0x20, 6, 0x73, 0x20, 7, 0x73, 0x0b)
				m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
				var initial []byte
				setup := func(mem []byte) {
					for i := 0; i < len(mem); i += 4 {
						binary.LittleEndian.PutUint32(mem[i:], uint32(i)*0x9e3779b9^0xffffffff)
					}
					initial = append([]byte{}, mem...)
				}
				saved := integerPacketAssertFast
				integerPacketAssertFast = count >= 2 && base == 0
				var stats ModuleStats
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true}}, setup, uint64(base), 512, uint64(count), 0, 17, 48)
				integerPacketAssertFast = saved
				index, sum, last, sign := uint32(0), uint32(17), uint32(0), uint32(0)
				trap := false
				for row := uint32(0); row < count && !trap; row++ {
					for lane := lanes - 1; lane >= 0; lane-- {
						a, b := uint64(base+index+uint32(lane*4)), uint64(512+index+uint32(lane*4))
						if a+4 > 65536 || b+4 > 65536 {
							trap = true
							break
						}
						last = binary.LittleEndian.Uint32(initial[a:]) - binary.LittleEndian.Uint32(initial[b:])
						sign = uint32(int32(last) >> 31)
						sum += (last ^ sign) - sign
					}
					index += 48
				}
				if (err != nil) != trap || !trap && got != uint64(sum^index^last^sign) || !bytes.Equal(mem, initial) {
					t.Fatalf("lanes=%d count=%d base=%d result=%x err=%v", lanes, count, base, got, err)
				}
				if stats.Funcs[0].Peephole["integer-packet-vector"] == 0 {
					t.Fatal("signed arithmetic packet was not emitted")
				}
			}
		}
	}
}

func TestIntegerPacketAcrossCalls(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := integerPacketModule(t, 8)
	callee := bytes.Repeat([]byte{0x01}, 4096)
	callee = append(callee, 0x41, 7, 0x0b)
	m.Code = append(m.Code, wasm.Func{BodyBytes: callee})
	m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
	var call []byte
	for i := 0; i < 6; i++ {
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
	for _, count := range []uint64{1, 2, 7} {
		want, expected, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-packet-vector": false}}, setup, 0, 512, count, 0, 17, 48)
		if err != nil {
			t.Fatal(err)
		}
		var stats ModuleStats
		saved := integerPacketAssertFast
		integerPacketAssertFast = count >= 2
		got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true}}, setup, 0, 512, count, 0, 17, 48)
		integerPacketAssertFast = saved
		if err != nil || got != want || !bytes.Equal(mem, expected) {
			t.Fatalf("count=%d got=%x want=%x err=%v", count, got, want, err)
		}
		if stats.Funcs[0].Peephole["integer-packet-vector"] == 0 || stats.Funcs[0].Calls["regabi"] == 0 {
			t.Fatalf("not a packet-bearing caller: %v %v", stats.Funcs[0].Peephole, stats.Funcs[0].Calls)
		}
	}
}

func TestIntegerPacketArithmetic(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, op := range []byte{0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76} {
		counts := []byte{3}
		if op >= 0x74 {
			counts = []byte{0, 1, 31, 32, 63}
		}
		for _, constant := range counts {
			m := integerPacketModule(t, 8)
			m.Code[0].BodyBytes = bytes.ReplaceAll(m.Code[0].BodyBytes, []byte{0x41, 3, 0x6c}, []byte{0x41, constant, op})
			var initial []byte
			setup := func(mem []byte) {
				for i := 0; i < len(mem); i += 4 {
					binary.LittleEndian.PutUint32(mem[i:], uint32(i)*0x9e3779b9^0x80000000)
				}
				initial = append([]byte{}, mem...)
			}
			var stats ModuleStats
			saved := integerPacketAssertFast
			integerPacketAssertFast = true
			got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-packet-vector": true}}, setup, 0, 512, 3, 0, 17, 48)
			integerPacketAssertFast = saved
			sum, temp := uint32(17), uint32(0)
			for row := uint32(0); row < 3; row++ {
				for lane := 7; lane >= 0; lane-- {
					i := row*48 + uint32(lane*4)
					v := binary.LittleEndian.Uint32(initial[i:]) - binary.LittleEndian.Uint32(initial[512+i:])
					c := uint32(constant)
					switch op {
					case 0x6a:
						v += c
					case 0x6b:
						v -= c
					case 0x6c:
						v *= c
					case 0x71:
						v &= c
					case 0x72:
						v |= c
					case 0x73:
						v ^= c
					case 0x74:
						v <<= c & 31
					case 0x75:
						v = uint32(int32(v) >> (c & 31))
					case 0x76:
						v >>= c & 31
					}
					v ^= 7
					sum += v
					if lane == 7 {
						temp = v
					}
				}
			}
			if err != nil || got != uint64(sum^144^temp) || !bytes.Equal(mem, initial) || stats.Funcs[0].Peephole["integer-packet-vector"] == 0 {
				t.Fatalf("op=%x constant=%d got=%x err=%v", op, constant, got, err)
			}
		}
	}
}
