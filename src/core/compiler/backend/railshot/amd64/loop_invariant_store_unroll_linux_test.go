//go:build linux && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestInvariantStoreUnrolledNative(t *testing.T) {
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*23 + 7)
		}
	}
	off := CompileOptions{Optimizations: map[string]bool{"loop-invariant-store": false}}
	on := CompileOptions{Optimizations: map[string]bool{"loop-invariant-store": true, "loop-invariant-store-unroll": true}}
	old := invariantStoreAssertFast
	defer func() { invariantStoreAssertFast = old }()
	for _, step := range []byte{1, 2, 4} {
		loop := invariantStoreLoopBody()
		at := bytes.Index(loop, []byte{0x20, 2, 0x41, 1, 0x6a})
		loop[at+3] = step
		at = bytes.Index(loop, []byte{0x41, 32, 0x47})
		loop = append(invariantConst(append([]byte{}, loop[:at]...), 0x41, 256), loop[at+2:]...)
		body := invariantConst([]byte{0, 0x02, 0x40, 0x20, 2}, 0x41, 256)
		body = append(body, 0x46, 0x0d, 0, 0x03, 0x40)
		body = append(body, loop...)
		body = append(body, 0x0b, 0x20, 3, 0x20, 2, 0x73, 0x20, 1, 0x73, 0x0b)
		m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
		rewritten, n, proofs := invariantStoreModuleWithLoadPolicy(m, true)
		if n != 1 || len(proofs[0]) != 2*invariantStoreUnrollFactor-1 {
			t.Fatal("unrolled prefix missing", n, proofs)
		}
		if err := wasm.ValidateModule(rewritten); err != nil {
			t.Fatal(err)
		}
		for count := uint64(0); count <= 33; count++ {
			for _, src := range []uint64{0, 1, 65404, 65408, 65532, 0xfffffff0} {
				for _, dst := range []uint64{0, 1, 512, 65532, 65533} {
					args := []uint64{dst, src, (256 - count*uint64(step)) | 0xdeadbeef00000000, 0xf1234567}
					want, wm, we := runMemAmd64WithOptions(t, m, off, init, args...)
					// Eligible disjoint, in-bounds fixtures must take the fast path.
					invariantStoreAssertFast = count >= 8 && src == 0 && dst == 512
					got, gm, ge := runMemAmd64WithOptions(t, m, on, init, args...)
					invariantStoreAssertFast = false
					if (we != nil) != (ge != nil) || we == nil && got != want || !bytes.Equal(wm, gm) {
						t.Fatalf("step=%d count=%d src=%x dst=%x got=%x/%v want=%x/%v memory=%v", step, count, src, dst, got, ge, want, we, bytes.Equal(wm, gm))
					}
				}
			}
		}
	}
}
