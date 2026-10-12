//go:build (linux || darwin) && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestArithmeticEqzDirectCompare(t *testing.T) {
	for _, wide := range []bool{false, true} {
		typ, sub, xor, eqz, eq := wasm.I32, byte(0x6b), byte(0x73), byte(0x45), byte(0x46)
		if wide {
			typ, sub, xor, eqz, eq = wasm.I64, 0x7d, 0x85, 0x50, 0x51
		}
		for _, op := range []byte{sub, xor} {
			for mode := 0; mode < 4; mode++ {
				prefix := []byte{0, 0x3f, 0, 0x1a, 0x20, 0, 0x20, 1}
				tail := []byte{0x0b}
				switch mode {
				case 1:
					tail = []byte{0x04, 0x7f, 0x41, 17, 0x05, 0x41, 23, 0x0b, 0x0b}
				case 2:
					tail = []byte{0x45, 0x0b}
				case 3:
					tail = []byte{0x22, 2, 0x0b}
				}
				body := append(append(append([]byte{}, prefix...), op, eqz), tail...)
				direct := append(append(append([]byte{}, prefix...), eq), tail...)
				makeMod := func(b []byte) *wasm.Module {
					m := mod1(t, []wasm.ValType{typ, typ, wasm.I32}, []wasm.ValType{wasm.I32}, b)
					m.Memories = []wasm.MemType{{Limits: wasm.Limits{Min: 1}}}
					return m
				}
				m := makeMod(body)
				cm, err := CompileModuleWith(m, CompileOptions{})
				if err != nil {
					t.Fatal(err)
				}
				reference, err := CompileModuleWith(makeMod(direct), CompileOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if cm.CodeImage != nil {
					defer cm.CodeImage.Close()
				}
				if reference.CodeImage != nil {
					defer reference.CodeImage.Close()
				}
				if len(cm.Code) > len(reference.Code) {
					t.Fatalf("wide=%v op=%x mode=%d arithmetic=%d direct=%d", wide, op, mode, len(cm.Code), len(reference.Code))
				}
				for _, x := range []uint64{0, 1, 0x80000000, 1 << 32, 0x8000000000000000, ^uint64(0)} {
					for _, y := range []uint64{0, 1, 0x80000000, 1 << 32, 0x8000000000000000, ^uint64(0)} {
						same := x == y
						if !wide {
							same = uint32(x) == uint32(y)
						}
						want := uint32(0)
						if same {
							want = 1
						}
						if mode == 1 {
							want = 23
							if same {
								want = 17
							}
						}
						if mode == 2 {
							want = 1 - want
						}
						got := uint32(runCompiledAmd64u(t, cm, x, y, 0))
						if got != want {
							t.Fatalf("wide=%v op=%x mode=%d x=%x y=%x got=%d want=%d", wide, op, mode, x, y, got, want)
						}
					}
				}
			}
		}
	}
}
