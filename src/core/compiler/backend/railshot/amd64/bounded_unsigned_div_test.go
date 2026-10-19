//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
	"testing"
)

func TestBoundedUnsignedReciprocalProof(t *testing.T) {
	for _, max := range []uint32{1, 7, 15, 63, 255, 1023, 4095, 65535} {
		for _, d := range []uint32{3, 5, 7, 9, 10, 11, 13, 17, 31, 63, 127, 255, 256, 257, 65535, 65537, 0x7fffffff} {
			m, s, ok := boundedUnsignedReciprocal(max, d)
			if !ok {
				continue
			}
			for n := uint32(0); n <= max; n++ {
				if q := uint32((uint64(n) * uint64(m)) >> s); q != n/d {
					t.Fatalf("max=%d d=%d n=%d multiplier=%d shift=%d got=%d", max, d, n, m, s, q)
				}
			}
		}
	}
	for _, x := range [][2]uint32{{0, 3}, {65536, 3}, {255, 0}, {255, 1}, {255, 2}, {255, 4}, {255, 0x80000000}} {
		if _, _, ok := boundedUnsignedReciprocal(x[0], x[1]); ok {
			t.Fatal("unsupported", x)
		}
	}
}

func TestBoundedUnsignedDivRangeExpression(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	// The unsigned shift proves a small dividend without an immediate AND.
	// Keep the original input live after the quotient/remainder is computed.
	for _, rem := range []bool{false, true} {
		op := byte(0x6e)
		if rem {
			op = 0x70
		}
		body := []byte{0, 0x20, 0, 0x41, 16, 0x76, 0x41, 7, op, 0x20, 0, 0x73, 0x0b}
		m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body)
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if stats.Funcs[0].Peephole["bounded-unsigned-div"] == 0 {
			t.Errorf("rem=%v lost unsigned-shift range", rem)
		}
		for _, input := range []uint64{0, 1, 65536, 0x7fffffff, 0xffffffff, 0xfedcba9876543210} {
			n := uint32(input)
			want := (n >> 16) / 7
			if rem {
				want = (n >> 16) % 7
			}
			if got := runCompiledAmd64u(t, cm, input); uint32(got) != want^n {
				t.Fatalf("rem=%v input=%x got=%x want=%x", rem, input, got, want^n)
			}
		}
	}
}

func TestBoundedUnsignedDivLocalRangeInvalidation(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, mutation := range []string{"none", "overwrite", "join", "wrap"} {
		for _, rem := range []bool{false, true} {
			body := []byte{0, 0x20, 0, 0x41, 0xff, 0x01, 0x71, 0x21, 1, 0x01}
			switch mutation {
			case "overwrite":
				body = append(body, 0x20, 0, 0x21, 1)
			case "join":
				body = append(body, 0x20, 2, 0x04, 0x40, 0x20, 0, 0x21, 1, 0x0b)
			case "wrap":
				body = append(body, 0x20, 1, 0x41, 0x80, 0x7c, 0x6a, 0x21, 1)
			}
			op := byte(0x6e)
			if rem {
				op = 0x70
			}
			body = append(body, 0x01, 0x20, 1, 0x41, 7, op, 0x0b)
			m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats})
			if err != nil {
				t.Fatal(err)
			}
			cm.CodeImage.Close()
			if (stats.Funcs[0].Peephole["bounded-unsigned-div"] != 0) != (mutation == "none") {
				t.Fatalf("mutation=%s rem=%v invalid range selection: %v", mutation, rem, stats.Funcs[0].Peephole)
			}
			for _, input := range []uint64{0, 255, 65536, 0xffffffff, 0xfedcba9876543210} {
				for _, condition := range []uint64{0, 1} {
					n := uint32(input) & 255
					if mutation == "overwrite" || mutation == "join" && condition != 0 {
						n = uint32(input)
					} else if mutation == "wrap" {
						n -= 512
					}
					want := n / 7
					if rem {
						want = n % 7
					}
					got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{}, nil, input, 0, condition)
					if err != nil || got != uint64(want) {
						t.Fatalf("mutation=%s rem=%v input=%x condition=%d got=%d want=%d err=%v", mutation, rem, input, condition, got, want, err)
					}
				}
			}
		}
	}
}

func TestBoundedUnsignedDivNative(t *testing.T) {
	saved := boundedUnsignedDivEnabled
	defer func() { boundedUnsignedDivEnabled = saved }()
	for _, mask := range []uint32{255, 65535, 0x800000ff} {
		for _, d := range []uint32{3, 5, 7, 9, 31, 65537} {
			for _, rem := range []bool{false, true} {
				op := byte(0x6e)
				if rem {
					op = 0x70
				}
				for _, swap := range []bool{false, true} {
					b := []byte{0}
					get := []byte{0x20, 0}
					c := append([]byte{0x41}, wasmtest.SLEB32(int32(mask))...)
					if swap {
						b = append(b, c...)
						b = append(b, get...)
					} else {
						b = append(b, get...)
						b = append(b, c...)
					}
					b = append(b, 0x71, 0x41)
					b = append(b, wasmtest.SLEB32(int32(d))...)
					b = append(b, op, 0xb)
					for _, on := range []bool{false, true} {
						boundedUnsignedDivEnabled = on
						m := mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, b)
						call, done := divInvoker(t, m)
						// Exhaust the full admitted domain and inject nonzero host upper bits.
						limit := mask
						if limit > 65535 {
							limit = 1023
						}
						for n := uint32(0); n <= limit; n++ {
							input := uint64(n) | 0xfedcba9800000000
							want := (n & mask) / d
							if rem {
								want = (n & mask) % d
							}
							if got := call(input); uint32(got) != want {
								t.Fatalf("mask=%x divisor=%d rem=%v swap=%v on=%v n=%x got=%x want=%x", mask, d, rem, swap, on, n, got, want)
							}
						}
						for _, n := range []uint32{0xffffffff, 0x80000000, 0xffff0000, 0x12345678} {
							want := (n & mask) / d
							if rem {
								want = (n & mask) % d
							}
							if got := call(uint64(n)); uint32(got) != want {
								t.Fatal("high operand", mask, d, on, n, got, want)
							}
						}
						done()
					}
				}
			}
		}
	}
}

func TestBoundedUnsignedDivNestedFixedRegisters(t *testing.T) {
	saved := boundedUnsignedDivEnabled
	defer func() { boundedUnsignedDivEnabled = saved }()
	for _, mask := range []uint32{255, 65535} {
		for _, innerRem := range []bool{false, true} {
			for _, outerRem := range []bool{false, true} {
				inner, outer := byte(0x6e), byte(0x6e)
				if innerRem {
					inner = 0x70
				}
				if outerRem {
					outer = 0x70
				}
				b := []byte{0, 0x20, 0, 0x41, 13, inner, 0x41}
				b = append(b, wasmtest.SLEB32(int32(mask))...)
				b = append(b, 0x71, 0x41, 7, outer, 0x20, 0, 0x73, 0x0b)
				for _, on := range []bool{false, true} {
					boundedUnsignedDivEnabled = on
					call, done := divInvoker(t, mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, b))
					for _, n := range []uint32{0, 1, 6, 7, 13, 255, 65535, 65536, 0x7fffffff, 0x80000000, 0xffffffff} {
						a := n / 13
						if innerRem {
							a = n % 13
						}
						a &= mask
						q := a / 7
						if outerRem {
							q = a % 7
						}
						want := q ^ n
						if got := call(uint64(n) | 0xabcd123400000000); uint32(got) != want {
							t.Fatal(mask, innerRem, outerRem, on, n, got, want)
						}
					}
					done()
				}
			}
		}
	}
}
