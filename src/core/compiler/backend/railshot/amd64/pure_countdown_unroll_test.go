//go:build linux && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func pureCountdownFixture(t *testing.T) *wasm.Module {
	// Local0=count,1=accumulator,2=coefficient,3=temporary. The enclosing
	// zero-trip guard belongs to this fixture, not to the rewrite's grammar.
	body := []byte{1, 1, 0x7f, 0x02, 0x40, 0x20, 0, 0x45, 0x0d, 0, 0x03, 0x40,
		0x20, 1, 0x20, 0, 0x6a, 0x20, 2, 0x6c, 0x22, 1,
		0x20, 1, 0x41, 3, 0x76, 0x73, 0x21, 3,
		0x20, 3, 0x20, 1, 0x20, 1, 0x41, 1, 0x71, 0x1b, 0x21, 1,
		0x20, 0, 0x41, 0x7f, 0x6a, 0x22, 0, 0x0d, 0, 0x0b, 0x0b,
		0x20, 1, 0x20, 0, 0x73, 0x20, 3, 0x73, 0x0b}
	return mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
}

func TestPureCountdownRewrite(t *testing.T) {
	m := pureCountdownFixture(t)
	before := append([]byte{}, m.Code[0].BodyBytes...)
	rewritten, n := pureCountdownModule(m)
	if n != 1 {
		t.Fatalf("rewrites=%d want1", n)
	}
	if !bytes.Equal(before, m.Code[0].BodyBytes) {
		t.Fatal("source mutated")
	}
	if len(rewritten.Code[0].BodyBytes) <= len(before) {
		t.Fatal("prefix missing")
	}
	if err := wasm.ValidateModule(rewritten); err != nil {
		t.Fatal(err)
	}
}

func TestPureCountdownNative(t *testing.T) {
	m := pureCountdownFixture(t)
	var code [][]byte
	for _, on := range []bool{false, true} {
		cm, err := CompileModuleWith(m, CompileOptions{Optimizations: map[string]bool{"pure-countdown-unroll": on}})
		if err != nil {
			t.Fatal(err)
		}
		code = append(code, append([]byte{}, cm.Code...))
		for n := uint32(0); n <= 65; n++ {
			for _, initial := range []uint32{0, 1, 0x80000000, 0xffffffff} {
				for _, coefficient := range []uint32{0, 1, 3, 0x87654321} {
					acc, tmp := initial, uint32(0)
					for count := n; count != 0; count-- {
						acc = (acc + count) * coefficient
						tmp = acc ^ (acc >> 3)
						if acc&1 != 0 {
							acc = tmp
						}
					}
					want := uint64(acc ^ tmp)
					if got := runCompiledAmd64u(t, cm, uint64(n)|0xdeadbeef00000000, uint64(initial)|0x1234567800000000, uint64(coefficient)|0xffffffff00000000); got != want {
						t.Fatalf("on=%v n=%d initial=%x coefficient=%x got=%x want=%x", on, n, initial, coefficient, got, want)
					}
				}
			}
		}
		cm.CodeImage.Close()
	}
	if bytes.Equal(code[0], code[1]) {
		t.Fatal("native rewrite not exercised")
	}
}
