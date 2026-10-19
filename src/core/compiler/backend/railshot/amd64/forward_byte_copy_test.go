//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestForwardByteCopyChangingAddressTemporary(t *testing.T) {
	// A temporary read before its new definition is a loop-carried value,
	// rather than an invariant. Its first-iteration affine stride is misleading.
	body := []byte{1, 1, 0x7f, 0x03, 0x40,
		0x20, 0, 0x20, 3, 0x6a, 0x20, 4, 0x6a,
		0x20, 1, 0x20, 3, 0x6a, 0x20, 4, 0x6a, 0x2d, 0, 0, 0x3a, 0, 0,
		0x20, 3, 0x21, 4, 0x20, 3, 0x41, 1, 0x6a, 0x21, 3,
		0x20, 2, 0x41, 1, 0x6b, 0x22, 2, 0x41, 0, 0x4b, 0x0d, 0, 0x0b,
		0x20, 3, 0x20, 4, 0x73, 0x0b}
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, option := range []string{"forward-byte-copy"} {
		var values [2]uint64
		var memory [2][]byte
		for i, on := range []bool{false, true} {
			var err error
			values[i], memory[i], err = runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{option: on}}, func(mem []byte) {
				for j := range mem {
					mem[j] = byte(j*17 + j/251 + 3)
				}
			}, 100, 1024, 128, 0)
			if err != nil {
				t.Fatal(err)
			}
		}
		if values[0] != values[1] || !bytes.Equal(memory[0], memory[1]) {
			t.Fatal("copy loop lost a changing address temporary")
		}
	}
}

func byteCopyTestBody(k int, latch byte) []byte {
	return byteCopyTestBodyOffsets(k, latch, 0, 0)
}

func byteCopyTestBodyOffsets(k int, latch byte, sourceOffset, destinationOffset uint64) []byte {
	// params: destination, source, remaining; locals: index, destination
	// temporary, source temporary. The return observes every modified local.
	b := []byte{1, 3, 0x7f, 0x03, 0x40,
		0x20, 0, 0x20, 3, 0x6a, 0x22, 4,
		0x20, 1, 0x20, 3, 0x6a, 0x22, 5}
	copyByte := func() {
		b = append(b, 0x2d, 0)
		b = binary.AppendUvarint(b, sourceOffset)
		b = append(b, 0x3a, 0)
		b = binary.AppendUvarint(b, destinationOffset)
	}
	copyByte()
	for i := 1; i < k; i++ {
		b = append(b, 0x20, 4, 0x41, byte(i), 0x6a,
			0x20, 5, 0x41, byte(i), 0x6a)
		copyByte()
	}
	b = append(b, 0x20, 3, 0x41, byte(k), 0x6a, 0x21, 3,
		0x20, 2, 0x41, byte(k), 0x6b, 0x22, 2, 0x41)
	c := byte(k - 1)
	if latch == 0x4f {
		c = byte(k)
	}
	if latch == 0x47 {
		c = 0
	}
	b = append(b, c, latch, 0x0d, 0, 0x0b,
		0x20, 2, 0x20, 3, 0x73, 0x20, 4, 0x73, 0x20, 5, 0x73, 0x0b)
	return b
}

func TestForwardByteCopy(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, k := range []int{1, 2, 3, 4, 7, 8, 16} {
		for _, latch := range []byte{0x4b, 0x4f, 0x47} {
			m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, byteCopyTestBody(k, latch))
			for _, pair := range [][2]uint64{{100, 1024}, {100, 100}, {101, 100}, {102, 100}, {107, 100}, {131, 100}, {100, 101}, {65530, 100}, {100, 65530}, {0xffffffff, 100}} {
				for _, count := range []uint64{uint64(k), uint64(k * 32), uint64(k*32 + 1)} {
					if latch == 0x47 && count%uint64(k) != 0 {
						continue
					}
					var got [2]uint64
					var memory [2][]byte
					var errs [2]error
					for i, on := range []bool{false, true} {
						var stats ModuleStats
						got[i], memory[i], errs[i] = runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"forward-byte-copy": on}}, func(mem []byte) {
							for j := range mem {
								mem[j] = byte(j*17 + j/251 + 3)
							}
						}, pair[0]|0xdeadbeef00000000, pair[1]|0xcafebabe00000000, count|0xffff000000000000)
						if (stats.Funcs[0].Peephole["forward-byte-copy"] > 0) != on {
							t.Fatalf("admission k=%d latch=%x on=%v: %v", k, latch, on, stats.Funcs[0].Peephole)
						}
					}
					if (errs[0] != nil) != (errs[1] != nil) || errs[0] == nil && got[0] != got[1] || !bytes.Equal(memory[0], memory[1]) {
						t.Fatalf("k=%d latch=%x pair=%v count=%d results=%x errors=%v", k, latch, pair, count, got, errs)
					}
				}
			}
		}
	}
}

func TestForwardByteCopyFastPath(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := forwardByteCopyAssertFast
	forwardByteCopyAssertFast = true
	defer func() { forwardByteCopyAssertFast = saved }()
	for _, k := range []int{1, 2, 3, 4, 7, 8, 16} {
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, byteCopyTestBody(k, 0x4b))
		for _, delta := range []uint64{0, 1, 2, 7, 31, 512} {
			count := uint64(k * 128)
			want := make([]byte, 65536)
			for i := range want {
				want[i] = byte(i*17 + i/251 + 3)
			}
			for i := uint64(0); i < count; i++ {
				want[100+delta+i] = want[100+i]
			}
			got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"forward-byte-copy": true}}, func(mem []byte) {
				for i := range mem {
					mem[i] = byte(i*17 + i/251 + 3)
				}
			}, 100+delta, 100, count)
			wantResult := count ^ (100 + delta + count - uint64(k)) ^ (100 + count - uint64(k))
			if err != nil || got != wantResult || !bytes.Equal(mem, want) {
				t.Fatalf("fast path k=%d overlap=%d: %v", k, delta, err)
			}
		}
	}
}

func TestForwardByteCopyMemoryImmediates(t *testing.T) {
	for _, offsets := range [][2]uint64{{0, 0}, {32, 17}, {0xfffffff0, 0}, {0, 0xfffffff0}} {
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, byteCopyTestBodyOffsets(3, 0x4b, offsets[0], offsets[1]))
		var values [2]uint64
		var memories [2][]byte
		var errors [2]error
		for i, on := range []bool{false, true} {
			values[i], memories[i], errors[i] = runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"forward-byte-copy": on}}, func(mem []byte) {
				for j := range mem {
					mem[j] = byte(j*17 + j/251 + 3)
				}
			}, 100, 101, 96)
		}
		if (errors[0] != nil) != (errors[1] != nil) || errors[0] == nil && values[0] != values[1] || !bytes.Equal(memories[0], memories[1]) {
			t.Fatalf("memory immediates=%v results=%v errors=%v", offsets, values, errors)
		}
	}
}
