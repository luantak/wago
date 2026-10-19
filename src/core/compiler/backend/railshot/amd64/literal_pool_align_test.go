//go:build linux && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	encoder "github.com/wago-org/wago/src/core/encoder/amd64"
)

func TestLiteralPoolMixedNaturalAlignment(t *testing.T) {
	requireCompilerDiagnostics(t)
	selection, err := optimizationBindings.ResolveSnapshot(map[string]bool{"literal-pool-align": true}, OptimizationSnapshot{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for n := 24; n < 40; n++ {
		f := fn{a: &encoder.Asm{B: make([]byte, n)}, stats: &CodegenStats{}, policy: shared.DefaultCodegenPolicy(selection), moduleHasSIMD: true}
		var payloads [][]byte
		for i, size := range []int{4, 16, 8, 4, 16, 8} {
			payload := bytes.Repeat([]byte{byte(0x91 + i)}, size)
			payloads = append(payloads, payload)
			f.recordConst(payload, i*4)
		}
		f.emitSIMDLiteralPool()
		for i, payload := range payloads {
			at := bytes.Index(f.a.B[n:], payload) + n
			if at < n || at%len(payload) != 0 {
				t.Fatalf("n=%d size=%d target=%d is not naturally aligned", n, len(payload), at)
			}
			target := i*4 + 4 + int(int32(binary.LittleEndian.Uint32(f.a.B[i*4:])))
			if target != at {
				t.Fatalf("relocation=%d want%d", target, at)
			}
		}
		if f.stats.NativeSize.LiteralPoolBytes != 56 {
			t.Fatalf("payload bytes=%d want56", f.stats.NativeSize.LiteralPoolBytes)
		}
	}
}

func TestLiteralPoolScalarOnlyLayoutUnchanged(t *testing.T) {
	var images [2][]byte
	for i, on := range []bool{false, true} {
		selection, err := optimizationBindings.ResolveSnapshot(map[string]bool{"literal-pool-align": on}, OptimizationSnapshot{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		f := fn{a: &encoder.Asm{B: make([]byte, 13)}, policy: shared.DefaultCodegenPolicy(selection), moduleHasSIMD: true}
		f.recordConst([]byte{1, 2, 3, 4}, 0)
		f.recordConst([]byte{5, 6, 7, 8, 9, 10, 11, 12}, 4)
		f.emitSIMDLiteralPool()
		images[i] = f.a.B
	}
	if !bytes.Equal(images[0], images[1]) {
		t.Fatal("vector alignment changed scalar-only pool")
	}
}

func TestLiteralPoolVectorAlignment(t *testing.T) {
	requireCompilerDiagnostics(t)
	const lo, hi = uint64(0x1122334455667788), uint64(0xfedcba9876543210)
	literal := make([]byte, 18)
	literal[0], literal[1] = 0xfd, 12
	binary.LittleEndian.PutUint64(literal[2:], lo)
	binary.LittleEndian.PutUint64(literal[10:], hi)
	unaligned := 0
	for _, on := range []bool{false, true} {
		for n := 0; n < 32; n++ {
			// Keep a real call so the constant preloader does not reserve an XMM
			// for this literal; this test targets the ordinary RIP-relative pool.
			body := []byte{0, 0x20, 0, 0x10, 1, 0x21, 0}
			for i := 0; i < n; i++ {
				body = append(body, 0x20, 0, 0x42, 1, 0x7c, 0x21, 0)
			}
			body = append(body, 0x20, 0, 0xfd, 0x12) // dynamic i64x2.splat
			body = append(body, literal...)
			body = append(body, 0xfd, 0x51, 0xfd, 0x1d, 0, 0x0b)
			m := modFuncs(t, funcDef{params: []wasm.ValType{wasm.I64}, results: []wasm.ValType{wasm.I64}, body: body}, funcDef{params: []wasm.ValType{wasm.I64}, results: []wasm.ValType{wasm.I64}, body: []byte{0, 0x20, 0, 0x42, 1, 0x7c, 0x0b}})
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"literal-pool-align": on, "inline": false}})
			if err != nil {
				t.Fatal(err)
			}
			if stats.NativeSize.LiteralPoolBytes != 16 || stats.NativeSize.LiteralPoolDuplicateBytes != 0 {
				t.Fatalf("padding counted as literal payload: %+v", stats.NativeSize)
			}
			at := bytes.Index(cm.Code, literal[2:])
			if at < 0 {
				t.Fatal("vector literal missing from native image")
			}
			if at%16 != 0 {
				unaligned++
				if on {
					t.Fatalf("enabled n=%d vector literal offset=%d is not16-byte aligned", n, at)
				}
			}
			for _, seed := range []uint64{0, 1, 0xffffffffffffffff, 0xabcdef0123456789} {
				want := lo ^ (seed + uint64(n) + 1)
				if got := runCompiledAmd64u(t, cm, seed); got != want {
					t.Fatalf("on=%v n=%d seed=%x got=%x want=%x", on, n, seed, got, want)
				}
			}
			if cm.CodeImage != nil {
				cm.CodeImage.Close()
			}
		}
	}
	if unaligned == 0 {
		t.Fatal("baseline did not exercise unaligned literal placement")
	}
}

func TestLiteralPoolAlignmentScalarAndCompact(t *testing.T) {
	requireCompilerDiagnostics(t)
	body := []byte{0, 0x44}
	body = binary.LittleEndian.AppendUint64(body, math.Float64bits(1.5))
	body = append(body, 0x44)
	body = binary.LittleEndian.AppendUint64(body, math.Float64bits(2.25))
	body = append(body, 0xa0, 0xbd, 0x0b)
	m := modFuncs(t, funcDef{results: []wasm.ValType{wasm.I64}, body: body}, funcDef{results: []wasm.ValType{wasm.I64}, body: body})
	for _, compact := range []bool{false, true} {
		var baseline, serial []byte
		for _, on := range []bool{false, true} {
			for _, workers := range []int{1, 2} {
				var stats ModuleStats
				cm, err := CompileModuleWith(m, CompileOptions{Workers: workers, CompactNative: compact, Stats: &stats, Optimizations: map[string]bool{"literal-pool-align": on}})
				if err != nil {
					t.Fatal(err)
				}
				if workers == 1 {
					serial = append([]byte(nil), cm.Code...)
				} else if !bytes.Equal(serial, cm.Code) {
					t.Fatal("serial/parallel pool layout mismatch")
				}
				if !on {
					baseline = append([]byte(nil), cm.Code...)
				} else if compact && !bytes.Equal(baseline, cm.Code) {
					t.Fatal("ordinary-pool flag changed compact module island")
				}
				wantBytes := 32
				if compact {
					wantBytes = 16
				}
				if stats.NativeSize.LiteralPoolBytes != wantBytes {
					t.Fatalf("compact=%v on=%v literal bytes=%d want%d", compact, on, stats.NativeSize.LiteralPoolBytes, wantBytes)
				}
				if got := runCompiledAmd64u(t, cm); got != math.Float64bits(3.75) {
					t.Fatalf("compact=%v on=%v got%x", compact, on, got)
				}
				if cm.CodeImage != nil {
					cm.CodeImage.Close()
				}
			}
		}
	}
}
