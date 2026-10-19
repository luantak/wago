//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestDeferredMemoryLeftSafetyBarriers(t *testing.T) {
	saved := deferredMemLeftEnabled
	deferredMemLeftEnabled = true
	defer func() { deferredMemLeftEnabled = saved }()
	for _, barrier := range []string{"signals", "shared", "eh", "regional", "division", "subtraction"} {
		left := testValueElem(memRefStorage(RAX, 0, 4, false, false, -1))
		right := testDeferredElem(opMul, mtI32, testValueElem(storage{kind: stConst, typ: mtI32, cval: 7}), testValueElem(storage{kind: stConst, typ: mtI32, cval: 3}))
		node := testDeferredElem(opXor, mtI32, left, right)
		f := &fn{policy: currentCodegenPolicy()}
		switch barrier {
		case "signals":
			f.guardMode = true
		case "shared":
			f.threadedMemory0 = true
		case "eh":
			f.moduleEH = true
		case "regional":
			f.intervalReg = []Reg{R12}
		case "division":
			right.setDeferredOp(opDivU)
		case "subtraction":
			node.setDeferredOp(opSub)
		}
		// No assembler or allocator: admitting an unsafe case would attempt to
		// emit its RHS instead of leaving the original lowering untouched.
		f.prepareDeferredMemoryLeft(node)
		if !right.isDeferred() {
			t.Fatalf("crossed %s barrier", barrier)
		}
	}
}

func TestDeferredMemoryLeft(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved, scalar := deferredMemLeftEnabled, sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { deferredMemLeftEnabled, sharedScalarEnabled = saved, scalar }()
	for _, wide := range []bool{false, true} {
		ty, load, c, mul, xor := wasm.I32, byte(0x28), byte(0x41), byte(0x6c), byte(0x73)
		if wide {
			ty, load, c, mul, xor = wasm.I64, 0x29, 0x42, 0x7e, 0x85
		}
		for _, secondLoad := range []bool{false, true} {
			body := []byte{0, 0x20, 0, load, 0, 0}
			if secondLoad {
				body = append(body, 0x41, 8, load, 0, 0)
			} else {
				body = append(body, 0x20, 1)
			}
			body = append(body, c, 3, mul, xor, 0x0b)
			m := modMem(t, 1, []wasm.ValType{wasm.I32, ty}, []wasm.ValType{ty}, body)
			for _, enabled := range []bool{false, true} {
				deferredMemLeftEnabled = enabled
				for _, raw := range []uint64{0, 4, 0xdeadbeef00000004, 65532, 65535, 0xffffffff} {
					var stats ModuleStats
					got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats}, func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i)
						}
					}, raw, 0x87654321fedcba98)
					size := uint64(4)
					if wide {
						size = 8
					}
					addr := uint64(uint32(raw))
					if addr+size > 65536 {
						if err == nil {
							t.Fatal("missing bounds trap")
						}
					} else {
						read := func(a uint64) uint64 {
							var v uint64
							for i := uint64(0); i < size; i++ {
								v |= uint64(byte(a+i)) << (i * 8)
							}
							return v
						}
						v := uint64(0x87654321fedcba98)
						if secondLoad {
							v = read(8)
						}
						want := read(addr) ^ (v * 3)
						if !wide {
							want = uint64(uint32(want))
						}
						if err != nil || got != want {
							t.Fatalf("wide=%v load=%v enabled=%v raw=%x got=%x want=%x err=%v", wide, secondLoad, enabled, raw, got, want, err)
						}
					}
					if enabled && stats.Funcs[0].Peephole["deferred-mem-left"] == 0 {
						t.Fatal("deferred memory fold not exercised")
					}
				}
			}
		}
	}
}
