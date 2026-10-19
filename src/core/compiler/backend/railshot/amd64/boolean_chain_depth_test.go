//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestBooleanChainDepth(t *testing.T) {
	requireCompilerDiagnostics(t)
	saved := sharedScalarEnabled
	sharedScalarEnabled = false
	defer func() { sharedScalarEnabled = saved }()
	for _, n := range []int{7, 9, 16, 17, 31} {
		for _, wide := range []bool{false, true} {
			for _, or := range []bool{false, true} {
				typ, cmp, combine := wasm.I32, byte(0x48), byte(0x71)
				if wide {
					typ, cmp = wasm.I64, 0x53
				}
				if or {
					combine = 0x72
				}
				params := make([]wasm.ValType, n+1)
				for i := range params {
					params[i] = typ
				}
				body := []byte{0}
				for i := 0; i < n; i++ {
					body = append(body, 0x20, byte(i), 0x20, byte(n), cmp)
					if i > 0 {
						body = append(body, combine)
					}
				}
				body = append(body, 0x0b)
				m := mod1(t, params, []wasm.ValType{wasm.I32}, body)
				for _, enabled := range []bool{false, true} {
					var stats ModuleStats
					cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"boolean-chain": true, "boolean-chain-depth": enabled, "boolean-chain-operands": true}})
					if err != nil {
						t.Fatal(err)
					}
					if (stats.Funcs[0].Peephole["boolean-chain-depth"] > 0) != enabled {
						t.Fatalf("n=%d enabled=%v stats=%v", n, enabled, stats.Funcs[0].Peephole)
					}
					for decisive := -1; decisive < n; decisive++ {
						args := make([]uint64, n+1)
						for i := 0; i < n; i++ {
							args[i] = 1
							if or {
								args[i] = 9
							}
						}
						args[n] = 7
						if decisive >= 0 {
							args[decisive] = 9
							if or {
								args[decisive] = 1
							}
						}
						want := uint64(0)
						if !or && decisive < 0 || or && decisive >= 0 {
							want = 1
						}
						if got := runCompiledAmd64u(t, cm, args...); got != want {
							t.Fatalf("n=%d wide=%v or=%v enabled=%v decisive=%d got=%d want=%d", n, wide, or, enabled, decisive, got, want)
						}
					}
					cm.CodeImage.Close()
				}
			}
		}
	}
}

func TestBooleanChainDepthPressureAndTrap(t *testing.T) {
	saved := booleanChainDepthEnabled
	booleanChainDepthEnabled = true
	defer func() { booleanChainDepthEnabled = saved }()
	TestBooleanChainPressureAndTrap(t)
}

func TestBooleanChainDepthLiveValues(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		body := []byte{0}
		for i := byte(1); i <= 24; i++ {
			body = append(body, 0x20, 0, 0x41, i, 0x6c)
		}
		for i := byte(1); i <= 9; i++ {
			body = append(body, 0x20, 0, 0x41, i, 0x48)
			if i > 1 {
				body = append(body, 0x71)
			}
		}
		for i := 0; i < 24; i++ {
			body = append(body, 0x73)
		}
		body = append(body, 0x0b)
		cm, err := CompileModuleWith(mod1(t, []wasm.ValType{wasm.I32}, []wasm.ValType{wasm.I32}, body), CompileOptions{Optimizations: map[string]bool{"boolean-chain": true, "boolean-chain-depth": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range []uint64{0, 1, 7, 9, 0xffffffff, 0xdeadbeef80000000} {
			n := uint32(input)
			want := uint32(0)
			if int32(n) < 1 {
				want = 1
			}
			for i := uint32(1); i <= 24; i++ {
				want ^= n * i
			}
			if got := uint32(runCompiledAmd64u(t, cm, input)); got != want {
				t.Fatalf("enabled=%v input=%x got=%x want=%x", enabled, input, got, want)
			}
		}
		cm.CodeImage.Close()
	}
}
