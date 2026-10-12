//go:build (linux || darwin) && arm64

package arm64

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
			for form := 0; form < 3; form++ {
				for mode := 0; mode < 5; mode++ {
					prefix := []byte{0, 0x3f, 0, 0x1a, 0x20, 0, 0x20, 1}
					if form == 1 {
						prefix = []byte{0, 0x3f, 0, 0x1a, 0x20, 0, 0x41, 0x7f}
						if wide {
							prefix[6] = 0x42
						}
					}
					if form == 2 {
						prefix = []byte{0, 0x3f, 0, 0x1a, 0x41, 0x7f, 0x20, 1}
						if wide {
							prefix[4] = 0x42
						}
					}
					tail := []byte{0x0b}
					switch mode {
					case 1:
						tail = []byte{0x04, 0x7f, 0x41, 17, 0x05, 0x41, 23, 0x0b, 0x0b}
					case 2:
						tail = []byte{0x45, 0x0b}
					case 3:
						tail = []byte{0x22, 2, 0x0b}
					case 4:
						tail = []byte{0x45, 0x04, 0x7f, 0x41, 23, 0x05, 0x41, 17, 0x0b, 0x0b}
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
							a, b := x, y
							if form == 1 {
								b = ^uint64(0)
							}
							if form == 2 {
								a = ^uint64(0)
							}
							same := a == b
							if !wide {
								same = uint32(a) == uint32(b)
							}
							want := uint32(0)
							if same {
								want = 1
							}
							if mode == 1 || mode == 4 {
								want = 23
								if same {
									want = 17
								}
							}
							if mode == 2 {
								want = 1 - want
							}
							raw, err := runArm64WrapperWithOptions(t, m, CompileOptions{}, x, y, 0)
							if err != nil {
								t.Fatal(err)
							}
							got := uint32(raw)
							if got != want {
								t.Fatalf("wide=%v op=%x mode=%d x=%x y=%x got=%d want=%d", wide, op, mode, x, y, got, want)
							}
						}
					}
				}
			}
		}
	}
}
