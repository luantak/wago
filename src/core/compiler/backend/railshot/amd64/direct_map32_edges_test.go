//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

type directMap32Run struct {
	eng             *coreruntime.Engine
	jm              *coreruntime.JobMemory
	args, out, trap []byte
	entry           uintptr
}

func newDirectMap32Run(t testing.TB, m *wasm.Module, opts CompileOptions, size, index int) *directMap32Run {
	t.Helper()
	cm, err := CompileModuleWith(m, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cm.CodeImage.Close() })
	eng, err := coreruntime.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	jm, err := coreruntime.NewJobMemory(size)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { jm.Close() })
	ar, err := coreruntime.NewArena(4096)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ar.Close() })
	code, entry, err := coreruntime.MapCode(cm.Code)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { coreruntime.Unmap(code) })
	callEntry := entry + uintptr(cm.Entry[index])
	return &directMap32Run{eng: eng, jm: jm, args: ar.Alloc(256), out: ar.Alloc(32), trap: ar.Alloc(coreruntime.TrapBufferBytes), entry: callEntry}
}

func (r *directMap32Run) call(args ...uint64) error {
	for i, v := range args {
		binary.LittleEndian.PutUint64(r.args[8*i:], v)
	}
	return r.eng.Call(r.entry, r.args, r.jm.LinearMemory(), r.trap, r.out)
}

func TestDirectMap32FastExecution(t *testing.T) {
	saved := regionLoopTestFast
	defer func() { regionLoopTestFast = saved }()
	regionLoopTestFast = true
	for _, fp := range []bool{false, true} {
		for _, n := range []uint64{32, 33, 512, 8192} {
			for _, pair := range [][2]uint64{{0, 32768}, {1, 32769}, {128, 128}} {
				if pair[1]+n*4 > 65536 {
					continue
				}
				setup := func(mem []byte) {
					for i := uint64(0); i < n; i++ {
						binary.LittleEndian.PutUint32(mem[pair[1]+i*4:], uint32(i))
					}
				}
				if _, _, err := runMemAmd64WithOptions(t, directMap32Fixture(t, fp, false), CompileOptions{}, setup, pair[0], pair[1], 0, n, 3, 7); err != nil {
					t.Fatal("guard rejected a qualified input", fp, n, pair, err)
				}
			}
		}
	}
}

func TestDirectMap32FloatRounding(t *testing.T) {
	a := newDirectMap32Run(t, directMap32Fixture(t, true, true), CompileOptions{}, 65536, 0)
	b := newDirectMap32Run(t, directMap32Fixture(t, true, false), CompileOptions{}, 65536, 0)
	values := [...]uint32{0, 0x80000000, 1, 0x80000001, 0x3f800001, 0x3f7fffff, 0x7f800000, 0xff800000, 0x7fc12345, 0x7f812345, 0xffffffff, 0x00800000}
	for _, scale := range []uint32{0, 0x80000000, 1, 0x3f800001, 0xbf800000, 0x7f800000, 0x7fc12345} {
		for _, bias := range []uint32{0, 0x80000000, 0xbf800000, 1, 0x7fc54321} {
			for _, r := range []*directMap32Run{a, b} {
				for i := 0; i < 64; i++ {
					binary.LittleEndian.PutUint32(r.jm.CurrentBytes()[32768+4*i:], values[i%len(values)])
				}
			}
			if err := a.call(0, 32768, 0, 64, uint64(scale), uint64(bias)); err != nil {
				t.Fatal(err)
			}
			if err := b.call(0, 32768, 0, 64, uint64(scale), uint64(bias)); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 64; i++ {
				want := binary.LittleEndian.Uint32(a.jm.CurrentBytes()[4*i:])
				got := binary.LittleEndian.Uint32(b.jm.CurrentBytes()[4*i:])
				if !math.IsNaN(float64(math.Float32frombits(want))) {
					if got != want {
						t.Fatalf("rounding scale=%x bias=%x i=%d got=%x want=%x", scale, bias, i, got, want)
					}
				} else if !math.IsNaN(float64(math.Float32frombits(got))) || got&0x400000 == 0 {
					t.Fatalf("nonpermitted NaN %x", got)
				}
			}
		}
	}
}
