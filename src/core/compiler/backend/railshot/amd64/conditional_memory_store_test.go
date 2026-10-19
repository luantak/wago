//go:build linux && amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func conditionalStoreCompare(op byte, a, b uint64, wide bool) bool {
	sa, sb := int64(a), int64(b)
	if !wide {
		a, b = uint64(uint32(a)), uint64(uint32(b))
		sa, sb = int64(int32(a)), int64(int32(b))
	}
	switch op {
	case 0x46:
		return a == b
	case 0x47:
		return a != b
	case 0x48:
		return sa < sb
	case 0x49:
		return a < b
	case 0x4a:
		return sa > sb
	case 0x4b:
		return a > b
	case 0x4c:
		return sa <= sb
	case 0x4d:
		return a <= b
	case 0x4e:
		return sa >= sb
	case 0x4f:
		return a >= b
	}
	panic("invalid comparison")
}

func TestConditionalMemoryStore(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, wide := range []bool{false, true} {
		ty, load, store, size := wasm.I32, byte(0x28), byte(0x36), uint64(4)
		if wide {
			ty, load, store, size = wasm.I64, 0x29, 0x37, 8
		}
		for cmp := byte(0x46); cmp <= 0x4f; cmp++ {
			for _, oldLeft := range []bool{false, true} {
				body := []byte{0, 0x02, 0x40}
				if oldLeft {
					body = append(body, 0x20, 0, load, 0, 4, 0x20, 1)
				} else {
					body = append(body, 0x20, 1, 0x20, 0, load, 0, 4)
				}
				actualCmp := cmp
				if wide {
					actualCmp += 11
				}
				body = append(body, actualCmp, 0x0d, 0, 0x20, 0, 0x20, 1, store, 0, 4, 0x0b, 0x20, 0, load, 0, 4, 0x0b)
				m := modMem(t, 1, []wasm.ValType{wasm.I32, ty}, []wasm.ValType{ty}, body)
				for _, pair := range [][2]uint64{{0, 0}, {1, 7}, {7, 1}, {0x80000000, 0xffffffff}, {0x8000000000000000, ^uint64(0)}} {
					for _, raw := range []uint64{0xdeadbeef00000000, 65524, 65529, 0xffffffff} {
						addr := uint64(uint32(raw)) + 4
						setup := func(mem []byte) {
							mem[0] = 0x5a
							if addr+size <= uint64(len(mem)) {
								for i := uint64(0); i < size; i++ {
									mem[addr+i] = byte(pair[0] >> (i * 8))
								}
							}
						}
						for _, on := range []bool{false, true} {
							var stats ModuleStats
							got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"conditional-memory-store": on}}, setup, raw, pair[1])
							wantMemory := make([]byte, 65536)
							setup(wantMemory)
							if addr+size > 65536 {
								if err == nil {
									t.Fatal("missing load bounds trap")
								}
							} else {
								a, b := pair[1], pair[0]
								if oldLeft {
									a, b = b, a
								}
								want := pair[0]
								if !conditionalStoreCompare(cmp, a, b, wide) {
									want = pair[1]
								}
								if !wide {
									want = uint64(uint32(want))
								}
								for i := uint64(0); i < size; i++ {
									wantMemory[addr+i] = byte(want >> (i * 8))
								}
								if err != nil || got != want {
									t.Fatalf("wide=%v cmp=%x oldLeft=%v on=%v pair=%x raw=%x got=%x want=%x err=%v", wide, cmp, oldLeft, on, pair, raw, got, want, err)
								}
							}
							if !bytes.Equal(mem, wantMemory) {
								t.Fatal("memory mismatch")
							}
							if on && stats.Funcs[0].Peephole["conditional-memory-store"] == 0 {
								t.Fatal("conditional store was not exercised")
							}
						}
					}
				}
			}
		}
	}
}

func TestConditionalMemoryStoreRejectsDifferentAccess(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, shape := range []string{"offset", "address", "value", "width"} {
		load, storeAddress, storeValue := byte(0x28), byte(0), byte(1)
		addr, other := uint64(0), uint64(123)
		if shape == "address" {
			storeAddress, other = 2, 65536
		}
		if shape == "value" {
			storeValue = 2
		}
		if shape == "width" {
			load, addr = 0x2d, 65535
		}
		body := []byte{0, 0x02, 0x40, 0x20, 1, 0x20, 0, load, 0, 0, 0x4f, 0x0d, 0, 0x20, storeAddress, 0x20, storeValue, 0x36, 0}
		if shape == "offset" {
			body = append(body, 0x80, 0x80, 4)
		} else {
			body = append(body, 0)
		}
		body = append(body, 0x0b, 0x20, 0, load, 0, 0, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, value := range []uint64{1, 9} {
			for _, on := range []bool{false, true} {
				var stats ModuleStats
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"conditional-memory-store": on}}, func(mem []byte) { mem[addr] = 7 }, addr, value, other)
				trap := value == 1 && shape != "value"
				want := uint64(7)
				if value == 1 && shape == "value" {
					want = 123
				}
				if (err != nil) != trap || err == nil && got != want || mem[addr] != byte(want) {
					t.Fatalf("shape=%s on=%v value=%d got=%d want=%d err=%v", shape, on, value, got, want, err)
				}
				if stats.Funcs[0].Peephole["conditional-memory-store"] != 0 {
					t.Fatalf("admitted mismatched %s", shape)
				}
			}
		}
	}
}

func TestConditionalMemoryStoreOperandPrefix(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, memoryPrefix := range []bool{false, true} {
		body := []byte{0}
		if memoryPrefix {
			body = append(body, 0x20, 0, 0x28, 0, 0)
		} else {
			body = append(body, 0x20, 2)
		}
		body = append(body, 0x02, 0x40, 0x20, 1, 0x20, 0, 0x28, 0, 0, 0x4f, 0x0d, 0, 0x20, 0, 0x20, 1, 0x36, 0, 0, 0x0b, 0x20, 0, 0x28, 0, 0, 0x73, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		for _, value := range []uint64{1, 9} {
			for _, on := range []bool{false, true} {
				var stats ModuleStats
				got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"conditional-memory-store": on}}, func(mem []byte) { mem[4] = 7 }, 0xdeadbeef00000004, value, 0xaabbccdd)
				selected := value
				if value >= 7 {
					selected = 7
				}
				prefix := uint64(0xaabbccdd)
				if memoryPrefix {
					prefix = 7
				}
				if err != nil || got != prefix^selected || mem[4] != byte(selected) {
					t.Fatalf("memoryPrefix=%v on=%v value=%d got=%x err=%v", memoryPrefix, on, value, got, err)
				}
				if on && stats.Funcs[0].Peephole["conditional-memory-store"] == 0 {
					t.Fatal("prefix fixture did not exercise conditional store")
				}
			}
		}
	}
}
