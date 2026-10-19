//go:build linux && amd64

package amd64

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestMemoryMinimumAnonymousTee(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, overwrite := range []bool{false, true} {
		body := []byte{1, 65, 0x7f, 0x20, 0, 0x41, 0xff, 0x01, 0x71, 0x22, 65}
		if overwrite {
			body = append(body, 0x20, 0, 0x21, 65)
		}
		body = append(body, 0x28, 0, 0, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if stats.Funcs[0].Peephole["memory-min-bounds"] != 1 {
			t.Fatalf("overwrite=%v lost anonymous tee range", overwrite)
		}
		for _, x := range []uint64{0, 255, 65536, 0xffffffff} {
			got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{}, func(mem []byte) {
				for i := range mem {
					mem[i] = byte(i*7 + 13)
				}
			}, x)
			i := uint32(x) & 255
			want := uint32(byte(i*7+13)) | uint32(byte((i+1)*7+13))<<8 | uint32(byte((i+2)*7+13))<<16 | uint32(byte((i+3)*7+13))<<24
			if err != nil || got != uint64(want) {
				t.Fatalf("x=%x got=%x want=%x err=%v", x, got, want, err)
			}
		}
	}
}

func TestMemoryMinimumAnonymousTeeInvalidation(t *testing.T) {
	for _, join := range []bool{false, true} {
		body := []byte{1, 65, 0x7f, 0x20, 0, 0x41, 0xff, 0x01, 0x71, 0x22, 65}
		if join {
			body = append(body, 0x1a, 0x20, 1, 0x04, 0x7f, 0x20, 0, 0x05, 0x41, 0, 0x0b)
		} else {
			// Erase the proof owner, then allocate an unrelated value. Arena
			// recycling must not lend the old proof to the reused address.
			body = append(body, 0x1a, 0x20, 0)
		}
		body = append(body, 0x28, 0, 0, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, x := range []uint64{0, 65532, 65533, 65536, 0xffffffff} {
			_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{}, nil, x, 1)
			if (err != nil) != (x > 65532) {
				t.Fatalf("join=%v x=%x err=%v", join, x, err)
			}
		}
	}
}

func TestMemoryMinimumLocalLoopTrapOrdering(t *testing.T) {
	// A pre-loop constant cannot justify the load on every iteration. The
	// second iteration must trap before its observable increment at address 8.
	body := []byte{0, 0x41, 0, 0x21, 1, 0x03, 0x40,
		0x20, 1, 0x28, 0, 0, 0x1a,
		0x41, 8, 0x41, 8, 0x28, 0, 0, 0x41, 1, 0x6a, 0x36, 0, 0,
		0x20, 0, 0x21, 1, 0x20, 2, 0x41, 1, 0x6b, 0x22, 2, 0x0d, 0,
		0x0b, 0x41, 42, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, enabled := range []bool{false, true} {
		for _, input := range []uint64{0, 65536, 0xffffffff} {
			_, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"memory-min-bounds": enabled}}, nil, input, 0, 2)
			want := uint32(2)
			if input != 0 {
				want = 1
			}
			if (err != nil) != (input != 0) || binary.LittleEndian.Uint32(mem[8:]) != want {
				t.Fatalf("on=%v input=%x trap=%v writes=%d want=%d", enabled, input, err, binary.LittleEndian.Uint32(mem[8:]), want)
			}
		}
	}
}

func TestMemoryMinimumBoundsLocalProof(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		typ, c, and, add := wasm.I32, byte(0x41), byte(0x71), byte(0x6a)
		if wide {
			typ, c, and, add = wasm.I64, 0x42, 0x83, 0x7c
		}
		for _, tee := range []bool{false, true} {
			for _, pin := range []bool{false, true} {
				t.Run(fmt.Sprintf("wide=%v/tee=%v/pin=%v", wide, tee, pin), func(t *testing.T) {
					body := []byte{1, 1, 0x7f}
					if !pin {
						body[1] = 65
					}
					body[2] = 0x7f
					if wide {
						body[2] = 0x7e
					}
					body = append(body, 0x20, 0, c, 0xff, 0x01, and)
					if tee {
						body = append(body, 0x22, 1, 0x1a)
					} else {
						body = append(body, 0x21, 1)
					}
					// Keep this as a local round trip, not the adjacent set/get
					// fold whose unpinned result remains an anonymous register.
					body = append(body, 0x01, 0x20, 1, c, 4, add, 0x28, 0, 0, 0x0b)
					m := modMem(t, 1, []wasm.ValType{typ}, []wasm.ValType{wasm.I32}, body)
					m.Memories[0].Limits.Addr64 = wide
					var stats ModuleStats
					cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats})
					if err != nil {
						t.Fatal(err)
					}
					defer cm.CodeImage.Close()
					if stats.Funcs[0].Peephole["memory-min-bounds"] == 0 {
						t.Fatalf("lost local range: %v", stats.Funcs[0].Peephole)
					}
					for _, input := range []uint64{0, 255, 65536, ^uint64(0)} {
						_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{}, nil, input)
						if err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func TestMemoryMinimumLocalProofInvalidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutation []byte
	}{
		{"overwrite", []byte{0x20, 0, 0x21, 1}},
		{"join", []byte{0x20, 2, 0x04, 0x40, 0x20, 0, 0x21, 1, 0x05, 0x41, 0, 0x21, 1, 0x0b}},
		{"backedge", []byte{0x03, 0x40, 0x20, 0, 0x21, 1, 0x20, 2, 0x41, 1, 0x6b, 0x22, 2, 0x0d, 0, 0x0b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]byte{0, 0x41, 0, 0x21, 1}, tc.mutation...)
			body = append(body, 0x20, 1, 0x28, 0, 0, 0x0b)
			m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
			for _, enabled := range []bool{false, true} {
				for _, input := range []uint64{0, 65532, 65533, 65536, 0xffffffff} {
					_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"memory-min-bounds": enabled}}, nil, input, 0, 2)
					if (err != nil) != (input > 65532) {
						t.Fatalf("on=%v input=%x trap=%v", enabled, input, err)
					}
				}
			}
		})
	}
}
