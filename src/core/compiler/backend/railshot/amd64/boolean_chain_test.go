//go:build linux && amd64

package amd64

import (
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestBooleanChain(t *testing.T) {
	requireCompilerDiagnostics(t)
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	for _, wide := range []bool{false, true} {
		for _, or := range []bool{false, true} {
			ty, lt, combine := wasm.I32, byte(0x48), byte(0x71)
			if wide {
				ty, lt = wasm.I64, 0x53
			}
			if or {
				combine = 0x72
			}
			body := []byte{0}
			for i := byte(0); i < 4; i++ {
				body = append(body, 0x20, i, 0x20, 4, lt)
				if i != 0 {
					body = append(body, combine)
				}
			}
			body = append(body, 0x0b)
			var stats ModuleStats
			cm, err := CompileModuleWith(mod1(t, []wasm.ValType{ty, ty, ty, ty, ty}, []wasm.ValType{wasm.I32}, body), CompileOptions{Stats: &stats, Optimizations: map[string]bool{"boolean-chain": true}})
			if err != nil {
				t.Fatal(err)
			}
			defer cm.CodeImage.Close()
			if stats.Funcs[0].Peephole["boolean-chain"] == 0 {
				t.Fatal("boolean chain not admitted")
			}
			for mask := 0; mask < 16; mask++ {
				args := []uint64{1, 1, 1, 1, 7}
				for i := 0; i < 4; i++ {
					if mask&(1<<i) != 0 {
						args[i] = 9
					}
				}
				want := uint64(0)
				if (!or && mask == 0) || (or && mask != 15) {
					want = 1
				}
				if got := runCompiledAmd64u(t, cm, args...); got != want {
					t.Fatalf("wide=%v or=%v mask=%x got=%d want=%d", wide, or, mask, got, want)
				}
			}
		}
	}
}

func TestBooleanChainPressureAndTrap(t *testing.T) {
	old := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = old }()
	body := []byte{0}
	for i := byte(1); i <= 24; i++ {
		body = append(body, 0x20, 0, 0x41, i, 0x6c)
	}
	for i := byte(0); i < 4; i++ {
		body = append(body, 0x20, i, 0x20, 4, 0x48)
		if i != 0 {
			body = append(body, 0x71)
		}
	}
	for i := 0; i < 24; i++ {
		body = append(body, 0x73)
	}
	body = append(body, 0x0b)
	m := mod1(t, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	for _, on := range []bool{false, true} {
		var stats ModuleStats
		opts := CompileOptions{Optimizations: map[string]bool{"boolean-chain": on}}
		if diagnosticsEnabled {
			opts.Stats = &stats
		}
		cm, err := CompileModuleWith(m, opts)
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if diagnosticsEnabled && on && stats.Funcs[0].Peephole["boolean-chain"] == 0 {
			t.Fatal("pressure case did not exercise boolean chain")
		}
		for _, args := range [][5]uint64{{1, 2, 3, 4, 7}, {9, 2, 3, 4, 7}, {1, 2, 9, 4, 7}, {0xffffffff, 2, 3, 4, 7}} {
			want := uint32(1)
			for i := 0; i < 4; i++ {
				if int32(args[i]) >= int32(args[4]) {
					want = 0
				}
			}
			for i := uint32(1); i <= 24; i++ {
				want ^= uint32(args[0]) * i
			}
			if got := runCompiledAmd64u(t, cm, args[:]...); got != uint64(want) {
				t.Fatalf("on=%v args=%x got=%x want=%x", on, args, got, want)
			}
		}
	}
	// A false earlier comparison cannot suppress a later mandatory load trap.
	trapBody := []byte{0, 0x20, 0, 0x41, 0, 0x48, 0x20, 1, 0x28, 0, 0, 0x45, 0x71, 0x20, 0, 0x41, 1, 0x48, 0x71, 0x0b}
	mem := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, trapBody)
	for _, on := range []bool{false, true} {
		_, _, err := runMemAmd64WithOptions(t, mem, CompileOptions{Optimizations: map[string]bool{"boolean-chain": on}}, nil, 10, 65536)
		if err == nil {
			t.Fatal("mandatory trap skipped")
		}
	}
}
