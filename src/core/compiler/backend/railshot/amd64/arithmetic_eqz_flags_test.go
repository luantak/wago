//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestArithmeticEqzFlags(t *testing.T) {
	for _, wide := range []bool{false, true} {
		for _, op := range []byte{0x6a, 0x6b, 0x72, 0x73} {
			typ, eqz := wasm.I32, byte(0x45)
			if wide {
				typ, eqz = wasm.I64, 0x50
				op += 0x12
			}
			// A memory.size forces the established compiler. XOR produces an owned
			// left operand for ADD; this avoids the existing LEA lowering.
			xor := byte(0x73)
			if wide {
				xor = 0x85
			}
			body := []byte{0, 0x3f, 0, 0x1a, 0x20, 0, 0x20, 1}
			if op != xor {
				body = append(body, xor, 0x20, 1)
			}
			body = append(body, op, eqz, 0x0b)
			m := mod1(t, []wasm.ValType{typ, typ}, []wasm.ValType{wasm.I32}, body)
			m.Memories = []wasm.MemType{{Limits: wasm.Limits{Min: 1}}}
			var stats ModuleStats
			opts := CompileOptions{}
			if diagnosticsEnabled {
				opts.Stats = &stats
			}
			cm, e := CompileModuleWith(m, opts)
			if e != nil {
				t.Fatal(e)
			}
			if cm.CodeImage != nil {
				defer cm.CodeImage.Close()
			}
			if diagnosticsEnabled && stats.Funcs[0].Peephole["arithmetic-eqz-flags"] != 1 {
				t.Fatalf("wide=%v op=%x missing flag reuse: %v", wide, op, stats.Funcs[0].Peephole)
			}
			for _, x := range []uint64{0, 1, 0x80000000, ^uint64(0)} {
				for _, y := range []uint64{0, 1, 0x80000000, ^uint64(0)} {
					v := x ^ y
					if op == xor {
						v = x
					}
					switch op {
					case 0x6a, 0x7c:
						v += y
					case 0x6b, 0x7d:
						v -= y
					case 0x72, 0x84:
						v |= y
					case 0x73, 0x85:
						v ^= y
					}
					if !wide {
						v = uint64(uint32(v))
					}
					want := uint64(0)
					if v == 0 {
						want = 1
					}
					if got := runCompiledAmd64u(t, cm, x, y); got != want {
						t.Fatalf("x=%x y=%x got=%x want=%x", x, y, got, want)
					}
				}
			}
		}
	}
}

func TestArithmeticEqzRetainsAffineLEA(t *testing.T) {
	for _, wide := range []bool{false, true} {
		typ, add, eqz := wasm.I32, byte(0x6a), byte(0x45)
		if wide {
			typ, add, eqz = wasm.I64, 0x7c, 0x50
		}
		body := []byte{0, 0x3f, 0, 0x1a, 0x20, 0, 0x20, 0, add, 0x20, 0, 0x20, 0, add, add, 0x20, 0, add, eqz, 0x0b}
		m := mod1(t, []wasm.ValType{typ}, []wasm.ValType{wasm.I32}, body)
		m.Memories = []wasm.MemType{{Limits: wasm.Limits{Min: 1}}}
		var s ModuleStats
		opts := CompileOptions{}
		if diagnosticsEnabled {
			opts.Stats = &s
		}
		cm, e := CompileModuleWith(m, opts)
		if e != nil {
			t.Fatal(e)
		}
		if cm.CodeImage != nil {
			defer cm.CodeImage.Close()
		}
		if diagnosticsEnabled && (s.Funcs[0].Peephole["arithmetic-eqz-flags"] != 0 || s.Funcs[0].Peephole["assoc-affine-add"] != 1) {
			t.Fatal("affine ADD was split", s.Funcs[0].Peephole)
		}
		for _, x := range []uint64{0, 1, 0x80000000, ^uint64(0)} {
			v := 5 * x
			if !wide {
				v = uint64(uint32(v))
			}
			want := uint64(0)
			if v == 0 {
				want = 1
			}
			if got := runCompiledAmd64u(t, cm, x); got != want {
				t.Fatalf("x=%x got=%d want=%d", x, got, want)
			}
		}
	}
}

func TestArithmeticEqzBranchFlags(t *testing.T) {
	for _, wide := range []bool{false, true} {
		for _, invert := range []bool{false, true} {
			typ, xor, add, eqz := wasm.I32, byte(0x73), byte(0x6a), byte(0x45)
			if wide {
				typ, xor, add, eqz = wasm.I64, 0x85, 0x7c, 0x50
			}
			body := []byte{0, 0x3f, 0, 0x1a, 0x20, 0, 0x20, 1, xor, 0x20, 1, add, eqz}
			if invert {
				body = append(body, 0x45)
			}
			body = append(body, 0x04, 0x7f, 0x41, 7, 0x05, 0x41, 9, 0x0b, 0x0b)
			m := mod1(t, []wasm.ValType{typ, typ}, []wasm.ValType{wasm.I32}, body)
			m.Memories = []wasm.MemType{{Limits: wasm.Limits{Min: 1}}}
			var stats ModuleStats
			opts := CompileOptions{}
			if diagnosticsEnabled {
				opts.Stats = &stats
			}
			cm, e := CompileModuleWith(m, opts)
			if e != nil {
				t.Fatal(e)
			}
			if cm.CodeImage != nil {
				defer cm.CodeImage.Close()
			}
			if diagnosticsEnabled && stats.Funcs[0].Peephole["arithmetic-eqz-flags"] != 1 {
				t.Fatal("missing branch flag reuse", stats.Funcs[0].Peephole)
			}
			for _, x := range []uint64{0, 1, 0x80000000, 0x8000000000000000, ^uint64(0)} {
				for _, y := range []uint64{0, 1, 0x80000000, ^uint64(0)} {
					v := (x ^ y) + y
					if !wide {
						v = uint64(uint32(v))
					}
					active := v == 0
					if invert {
						active = !active
					}
					want := uint64(9)
					if active {
						want = 7
					}
					if got := runCompiledAmd64u(t, cm, x, y); got != want {
						t.Fatalf("wide=%v invert=%v x=%x y=%x got=%d want=%d", wide, invert, x, y, got, want)
					}
				}
			}
		}
	}
}
