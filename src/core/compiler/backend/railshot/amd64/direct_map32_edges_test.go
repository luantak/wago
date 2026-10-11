//go:build linux && amd64

package amd64

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	enc "github.com/wago-org/wago/src/core/encoder/amd64"
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
	if cm.Entry[index] == cm.InternalEntry[index] {
		// The real Yosys routine has an internal register entry. Add only a native
		// host adapter; the Wasm module and its compiled function stay unchanged.
		ft, ok := m.LocalFuncType(index)
		if !ok || len(ft.Params) != 3 || ft.Params[0] != wasm.I32 || ft.Params[1] != wasm.F32 || ft.Params[2] != wasm.F32 || len(ft.Results) != 0 {
			t.Fatal("unsupported evidence adapter signature")
		}
		a := enc.Asm{}
		a.MovReg64(RBX, RSI)
		a.Load64(R15, RBX, -bdCurBytes)
		a.Push(RCX)
		a.FLoadDisp(fpArgRegs[0], RDI, 8, false)
		a.FLoadDisp(fpArgRegs[1], RDI, 16, false)
		a.Load64(intArgRegs[0], RDI, 0)
		a.MovImm64(R8, uint64(entry+uintptr(cm.InternalEntry[index])))
		a.CallReg(R8)
		a.Pop(RCX)
		a.Ret()
		shim, addr, err := coreruntime.MapCode(a.B)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { coreruntime.Unmap(shim) })
		callEntry = addr
	}
	return &directMap32Run{eng: eng, jm: jm, args: ar.Alloc(256), out: ar.Alloc(32), trap: ar.Alloc(coreruntime.TrapBufferBytes), entry: callEntry}
}

func (r *directMap32Run) call(args ...uint64) error {
	for i, v := range args {
		binary.LittleEndian.PutUint64(r.args[8*i:], v)
	}
	return r.eng.Call(r.entry, r.args, r.jm.LinearMemory(), r.trap, r.out)
}

func TestDirectMap32FullMemoryWrapAndOffsets(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, offset := range []byte{0, 4} {
			for _, storeOffset := range []bool{false, true} {
				source := directMap32Fixture(t, fp, true)
				candidate := directMap32Fixture(t, fp, false)
				load, store := byte(0x28), byte(0x36)
				if fp {
					load, store = 0x2a, 0x38
				}
				for _, m := range []*wasm.Module{source, candidate} {
					m.Memories[0].Limits.Min = 65536
					op := load
					if storeOffset {
						op = store
					}
					at := bytes.Index(m.Code[0].BodyBytes, []byte{op, 0, 0})
					if at < 0 {
						t.Fatal("fixture memory instruction missing")
					}
					m.Code[0].BodyBytes[at+2] = offset
				}
				opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}
				a := newDirectMap32Run(t, source, opts, 1<<32, 0)
				b := newDirectMap32Run(t, candidate, opts, 1<<32, 0)
				for _, pair := range [][2]uint64{{256, 0xfffffff0}, {0xfffffff0, 256}, {0xfffffff0, 0xfffffff0}, {0xfffffff1, 256}} {
					for _, r := range []*directMap32Run{a, b} {
						mem := r.jm.CurrentBytes()
						clear(mem[:512])
						clear(mem[0xffffffe0:])
						for i := 0; i < 64; i++ {
							binary.LittleEndian.PutUint32(mem[256+i*4:], uint32(i)+0x3f800001)
						}
						for i := 0; i < 8; i++ {
							binary.LittleEndian.PutUint32(mem[0xffffffe0+i*4:], uint32(i)+0x3f800001)
						}
					}
					scale, bias := uint64(0xffffffff), uint64(0x80000001)
					if fp {
						scale, bias = f32b(1.0000001), f32b(-1)
					}
					ae, be := a.call(pair[0], pair[1], 0, 17, scale, bias), b.call(pair[0], pair[1], 0, 17, scale, bias)
					am, bm := a.jm.CurrentBytes(), b.jm.CurrentBytes()
					if (ae == nil) != (be == nil) || !bytes.Equal(am[:512], bm[:512]) || !bytes.Equal(am[0xffffffe0:], bm[0xffffffe0:]) || ae == nil && !bytes.Equal(a.out, b.out) {
						t.Fatal(fp, offset, storeOffset, pair, ae, be)
					}
					if ae != nil {
						x := ae.(*coreruntime.TrapError)
						y := be.(*coreruntime.TrapError)
						if x.Code != y.Code {
							t.Fatal(x, y)
						}
					}
				}
			}
		}
	}
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

func TestDirectMap32RealNative(t *testing.T) {
	path := os.Getenv("DIRECT_MAP32_REAL")
	if path == "" {
		t.Skip("real module not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := wasm.DecodeModule(raw)
	if err != nil {
		t.Fatal(err)
	}
	var stats ModuleStats
	cm, err := CompileModuleWith(m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline, Stats: optionalTestStats(&stats)})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	selected := 0
	for i, x := range stats.Funcs {
		if x.Peephole["region-loop-map32"] != 0 {
			selected += x.Peephole["region-loop-map32"]
			t.Logf("function=%d selections=%d native=%d frame=%d spills=%d reloads=%d", i+m.ImportedFuncCount(), x.Peephole["region-loop-map32"], x.CodeBytes, x.FrameBytes, x.Spills, x.Reloads)
		}
	}
	t.Logf("module=%s bytes=%d selections=%d", path, len(cm.Code), selected)
}

func directMap32YosysModule(t testing.TB) *wasm.Module {
	path := os.Getenv("DIRECT_MAP32_YOSYS")
	if path == "" {
		t.Skip("Yosys path not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := wasm.DecodeModule(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func setupDirectMap32Yosys(mem []byte, n uint32) {
	// Unmodified Yosys function 5011 scales two flat arrays and one nested array.
	binary.LittleEndian.PutUint32(mem[72:], n)
	binary.LittleEndian.PutUint32(mem[76:], 4096)
	binary.LittleEndian.PutUint32(mem[84:], n)
	binary.LittleEndian.PutUint32(mem[88:], 65536)
	binary.LittleEndian.PutUint32(mem[96:], 1)
	binary.LittleEndian.PutUint32(mem[100:], 256)
	binary.LittleEndian.PutUint32(mem[256:], 512)
	binary.LittleEndian.PutUint32(mem[516:], n)
	binary.LittleEndian.PutUint32(mem[520:], 131072)
	for _, ptr := range []int{4096, 65536, 131072} {
		for i := uint32(0); i < n; i++ {
			binary.LittleEndian.PutUint32(mem[ptr+int(i)*4:], math.Float32bits(float32(i%1024)/1024))
		}
	}
}
func TestDirectMap32YosysExecution(t *testing.T) {
	m := directMap32YosysModule(t)
	opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}
	r := newDirectMap32Run(t, m, opts, 1<<20, 5011-m.ImportedFuncCount())
	for _, n := range []uint32{0, 1, 4, 8, 16, 17, 32, 33, 8192} {
		mem := r.jm.CurrentBytes()
		setupDirectMap32Yosys(mem, n)
		if err := r.call(64, f32b(1.25), f32b(-2)); err != nil {
			t.Fatal(n, err)
		}
		for j, ptr := range []int{4096, 65536, 131072} {
			for i := uint32(0); i < n; i++ {
				scale := float32(1.25)
				if j == 1 {
					scale = -2
				}
				got := binary.LittleEndian.Uint32(mem[ptr+int(i)*4:])
				want := math.Float32bits((float32(i%1024) / 1024) * scale)
				if got != want {
					t.Fatalf("n=%d stream=%d i=%d got=%x want=%x", n, j, i, got, want)
				}
			}
		}
		t.Logf("n=%d output SHA256=%x", n, sha256.Sum256(mem))
	}
}
func BenchmarkDirectMap32YosysExecute(b *testing.B) {
	m := directMap32YosysModule(b)
	r := newDirectMap32Run(b, m, CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}, 1<<20, 5011-m.ImportedFuncCount())
	for _, n := range []uint32{4, 16, 32, 64, 512, 8192} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setupDirectMap32Yosys(r.jm.CurrentBytes(), n)
			binary.LittleEndian.PutUint64(r.args, 64)
			binary.LittleEndian.PutUint64(r.args[8:], f32b(1))
			binary.LittleEndian.PutUint64(r.args[16:], f32b(1))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := r.eng.Call(r.entry, r.args, r.jm.LinearMemory(), r.trap, r.out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
