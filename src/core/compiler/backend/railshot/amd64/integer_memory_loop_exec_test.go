//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"golang.org/x/sys/cpu"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"github.com/wago-org/wago/tests/support/wasmtest"
)

func TestIntegerMemoryAVX2(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, wide := range []bool{false, true} {
		features := shared.AMD64ModernBaseline
		if wide {
			features |= shared.AMD64AVX2
		}
		opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: features, Optimizations: map[string]bool{"integer-memory-vector": true}}
		cm, err := CompileModuleWith(integerMemoryModule(t, 4), opts)
		if err != nil {
			t.Fatal(err)
		}
		gotWide := cm.RequiresAVX2
		cm.CodeImage.Close()
		if gotWide != wide {
			t.Fatalf("AVX2 requirement=%v want=%v", gotWide, wide)
		}
		if wide && !cpu.X86.HasAVX2 {
			continue
		}
		for _, step := range []byte{4, 8} {
			for n := uint64(0); n <= 33; n++ {
				m := integerMemoryModule(t, step)
				init := func(mem []byte) {
					for i := range mem {
						mem[i] = byte(i*37 + 19)
					}
				}
				want, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-memory-vector": false}}, init, 0, 512, n*uint64(step), 0)
				if err != nil {
					t.Fatal(err)
				}
				saved := integerMemoryAssertFast
				integerMemoryAssertFast = n >= 8
				got, _, err := runMemAmd64WithOptions(t, m, opts, init, 0, 512, n*uint64(step), 0)
				integerMemoryAssertFast = saved
				if err != nil || got != want {
					t.Fatalf("wide=%v step=%d n=%d got=%x want=%x err=%v", wide, step, n, got, want, err)
				}
			}
			// Full-width reads must not cross the memory boundary even when the
			// last scalar load is legal. A failed vector guard must preserve traps.
			for _, base := range []uint64{65472, 65476, 65504, 65508, 65532, 0xfffffffc} {
				for _, n := range []uint64{4, 7, 8, 9, 16, 17} {
					m := integerMemoryModule(t, step)
					init := func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*19 + 23)
						}
					}
					want, _, wantErr := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-memory-vector": false}}, init, base, base, n*uint64(step), 0)
					got, _, gotErr := runMemAmd64WithOptions(t, m, opts, init, base, base, n*uint64(step), 0)
					if (gotErr != nil) != (wantErr != nil) || (wantErr == nil && got != want) {
						t.Fatalf("boundary wide=%v step=%d base=%x n=%d got=%x/%v want=%x/%v", wide, step, base, n, got, gotErr, want, wantErr)
					}
				}
			}
		}
	}
}

func integerMemoryModule(t *testing.T, step byte) *wasm.Module {
	loop := integerMemoryLoopBody()
	pos := bytes.Index(loop, []byte{0x20, 3, 0x41, 4, 0x6a, 0x22, 3})
	loop[pos+3] = step
	body := []byte{1, 1, 0x7f, 0x02, 0x40, 0x20, 2, 0x20, 3, 0x46, 0x0d, 0, 0x03, 0x40}
	body = append(body, loop...)
	body = append(body, 0x0b, 0x20, 4, 0x0b)
	return modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
}

func TestIntegerMemoryVectorAdmission(t *testing.T) {
	old := integerMemoryVectorEnabled
	integerMemoryVectorEnabled = true
	defer func() { integerMemoryVectorEnabled = old }()
	requireCompilerDiagnostics(t)
	stats := compileWithStats(t, integerMemoryModule(t, 4), false).Funcs[0]
	if stats.Peephole["integer-memory-vector"] != 1 {
		t.Fatal("integer memory loop not vectorized")
	}
	if stats.Peephole["integer-memory-register-stream"] != 2 {
		t.Fatal("independent stream pointers were not kept in registers")
	}
}

func TestIntegerMemoryVectorExecution(t *testing.T) {
	old := integerMemoryVectorEnabled
	integerMemoryVectorEnabled = true
	defer func() { integerMemoryVectorEnabled = old }()
	for _, step := range []byte{4, 8} {
		for _, n := range []uint32{0, 1, 2, 3, 4, 5, 7, 8, 9, 17} {
			for _, bases := range [][3]uint32{{0, 128, 0}, {0xfffffffc, 0xfffffff8, 4}, {65504, 65504, 0}} {
				t.Run(fmt.Sprintf("step%d/n%d/base%x", step, n, bases[0]), func(t *testing.T) {
					saved := integerMemoryAssertFast
					integerMemoryAssertFast = n >= 4 && bases[0] == 0
					defer func() { integerMemoryAssertFast = saved }()
					start, limit := bases[2], bases[2]+n*uint32(step)
					var initial []byte
					init := func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*37 + 11)
						}
						initial = append([]byte{}, mem...)
					}
					got, mem, err := runMemAmd64WithOptions(t, integerMemoryModule(t, step), CompileOptions{}, init, uint64(bases[0])|0xdeadbeef00000000, uint64(bases[1]), uint64(limit), uint64(start))
					var want uint32
					trapped := false
					for i := uint32(0); i < n; i++ {
						x := start + i*uint32(step)
						a, b := uint64(bases[0]+x), uint64(bases[1]+x)+4
						if a+4 > uint64(len(initial)) || b+4 > uint64(len(initial)) {
							trapped = true
							break
						}
						want += (binary.LittleEndian.Uint32(initial[a:]) ^ 3) + (binary.LittleEndian.Uint32(initial[b:]) + 7)
					}
					if (err != nil) != trapped {
						t.Fatalf("trap=%v want=%v", err, trapped)
					}
					if !trapped && uint32(got) != want {
						t.Fatalf("got %x want %x", got, want)
					}
					if !bytes.Equal(mem, initial) {
						t.Fatal("read-only loop changed memory")
					}
				})
			}
		}
	}
}

func TestIntegerMemoryVectorFallbacks(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, mode := range []string{"disabled", "sse2", "shared"} {
		t.Run(mode, func(t *testing.T) {
			m := integerMemoryModule(t, 4)
			opts := CompileOptions{Optimizations: map[string]bool{"integer-memory-vector": mode != "disabled"}, AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}
			if mode == "sse2" {
				opts.AMD64Features = 0
			}
			if mode == "shared" {
				m.Memories[0].Shared = true
			}
			var stats ModuleStats
			opts.Stats = &stats
			cm, err := CompileModuleWith(m, opts)
			if err != nil {
				t.Fatal(err)
			}
			cm.CodeImage.Close()
			if stats.Funcs[0].Peephole["integer-memory-vector"] != 0 {
				t.Fatal("unsafe admission")
			}
		})
	}
	// Non-divisible and backward trip counts must enter the scalar body. That
	// original body eventually traps on these finite-memory inputs.
	for _, args := range [][4]uint64{{0, 128, 3, 0}, {0, 128, 4, 8}} {
		_, _, err := runMemAmd64WithOptions(t, integerMemoryModule(t, 4), CompileOptions{Optimizations: map[string]bool{"integer-memory-vector": true}}, nil, args[:]...)
		if err == nil {
			t.Fatal("invalid trip guard suppressed the scalar trap")
		}
	}
}

func TestIntegerMemoryVectorDeadLoadStillTraps(t *testing.T) {
	requireCompilerDiagnostics(t)
	m := integerMemoryModule(t, 4)
	m.Code[0].Locals.Runs[0].Count = 2
	body := m.Code[0].BodyBytes
	at := bytes.Index(body, []byte{3, 0x40}) + 2
	extra := []byte{0x20, 0, 0x41}
	extra = append(extra, wasmtest.SLEB32(65535)...)
	extra = append(extra, 0x6a, 0x28, 2, 0, 0x21, 5, 0x41, 0, 0x21, 5)
	m.Code[0].BodyBytes = append(append(append([]byte{}, body[:at]...), extra...), body[at:]...)
	var stats ModuleStats
	_, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-memory-vector": true}}, nil, 0, 128, 16, 0)
	if stats.Funcs[0].Peephole["integer-memory-vector"] != 1 {
		t.Fatal("test did not exercise guarded vector prefix")
	}
	if err == nil {
		t.Fatal("dead load trap was removed")
	}
}

func TestIntegerMemoryStreamHomes(t *testing.T) {
	requireCompilerDiagnostics(t)
	for _, mode := range []string{"coalesced-positive", "coalesced-negative", "three-streams", "invariant"} {
		for _, step := range []byte{4, 8} {
			t.Run(fmt.Sprintf("%s/step%d", mode, step), func(t *testing.T) {
				m := integerMemoryModule(t, step)
				body := append([]byte{}, m.Code[0].BodyBytes...)
				second := bytes.Index(body, []byte{0x20, 1, 0x20, 3, 0x6a, 0x28, 2, 4})
				switch mode {
				case "coalesced-positive", "coalesced-negative":
					body[second+1] = 0
					if mode == "coalesced-negative" {
						first := bytes.Index(body, []byte{0x20, 0, 0x20, 3, 0x6a, 0x28, 2, 0})
						body[first+7] = 20
					}
				case "three-streams":
					at := bytes.Index(body, []byte{0x6a, 0x6a, 0x21, 4}) + 2
					extra := []byte{0x20, 2, 0x20, 3, 0x6a, 0x28, 2, 0, 0x6a}
					body = append(append(append([]byte{}, body[:at]...), extra...), body[at:]...)
				case "invariant":
					body = append(append([]byte{}, body[:second+2]...), body[second+5:]...)
				}
				m.Code[0].BodyBytes = body
				for _, n := range []uint64{4, 5, 8, 17} {
					init := func(mem []byte) {
						for i := range mem {
							mem[i] = byte(i*13 + 71)
						}
					}
					want, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Optimizations: map[string]bool{"integer-memory-vector": false}}, init, 0, 128, n*uint64(step), 0)
					if err != nil {
						t.Fatal(err)
					}
					saved := integerMemoryAssertFast
					integerMemoryAssertFast = true
					var stats ModuleStats
					got, _, err := runMemAmd64WithOptions(t, m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"integer-memory-vector": true}}, init, 0, 128, n*uint64(step), 0)
					integerMemoryAssertFast = saved
					if err != nil || got != want {
						t.Fatalf("n=%d got=%x want=%x err=%v", n, got, want, err)
					}
					if stats.Funcs[0].Peephole["integer-memory-vector"] != 1 {
						t.Fatal("vector path not selected")
					}
					coalesced := mode == "coalesced-positive" || mode == "coalesced-negative"
					if (stats.Funcs[0].Peephole["integer-memory-register-stream"] == 1) != coalesced {
						t.Fatal("incorrect stream residency")
					}
					if (stats.Funcs[0].Peephole["integer-memory-coalesced-stream"] == 1) != coalesced {
						t.Fatal("incorrect stream coalescing")
					}
				}
			})
		}
	}
}
