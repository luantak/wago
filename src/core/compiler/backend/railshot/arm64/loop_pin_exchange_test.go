//go:build (linux || darwin || windows) && arm64

package arm64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math"
	"testing"
)

// A long first loop makes the function's pins favor variables inactive in the
// second loop. Its accumulator must borrow a pin, survive the backedge, and
// return that pin before the original variable is read and a call is made.
func loopPinScalarModule(t testing.TB, typ, loopType wasm.ValType) *wasm.Module {
	params := []wasm.ValType{wasm.I32}
	for range 30 {
		params = append(params, typ)
	}
	add := byte(0x6a)
	switch typ {
	case wasm.I64:
		add = 0x7c
	case wasm.F32:
		add = 0x92
	case wasm.F64:
		add = 0xa0
	}
	loopAdd := add
	var intoLoop, fromLoop []byte
	if typ != loopType {
		switch typ {
		case wasm.I32:
			loopAdd = 0x7c
			intoLoop = []byte{0xad}
			fromLoop = []byte{0xa7}
		case wasm.I64:
			loopAdd = 0x6a
			intoLoop = []byte{0xa7}
			fromLoop = []byte{0xad}
		case wasm.F32:
			loopAdd = 0xa0
			intoLoop = []byte{0xbb}
			fromLoop = []byte{0xb6}
		case wasm.F64:
			loopAdd = 0x92
			intoLoop = []byte{0xb6}
			fromLoop = []byte{0xbb}
		}
	}
	const counter, acc, temp = 31, 32, 33
	body := []byte{3, 1, 0x7f, 1, wasm.MustEncodeValType(typ), 1, wasm.MustEncodeValType(loopType), 0x41, 1, 0x21, counter, 3, 0x40}
	first := true
	for x := byte(1); x <= 30; x++ {
		for range 8 {
			body = append(body, 0x20, x)
			if !first {
				body = append(body, add)
			}
			first = false
		}
	}
	body = append(body, 0x21, acc, 0x20, counter, 0x41, 1, 0x6b, 0x22, counter, 0x0d, 0, 0x0b)
	body = append(body, 0x20, 30)
	body = append(body, intoLoop...)
	body = append(body, 0x21, temp, 0x20, 0, 0x21, counter,
		0x02, 0x40, 0x20, counter, 0x45, 0x0d, 0,
		3, 0x40, 0x20, temp, 0x20, temp, loopAdd, 0x20, temp, loopAdd, 0x21, temp,
		0x20, counter, 0x41, 1, 0x6b, 0x22, counter, 0x0d, 0, 0x0b, 0x0b,
		0x20, temp)
	body = append(body, fromLoop...)
	body = append(body, 0x20, acc, add, 0x20, 1, add, 0x10, 1, 0x0b)
	return modFuncs(t, funcDef{params, []wasm.ValType{typ}, body}, funcDef{[]wasm.ValType{typ}, []wasm.ValType{typ}, []byte{0, 0x20, 0, 0x0b}})
}

func TestLoopPinScalarWidthsAndExit(t *testing.T) { testLoopPinScalarWidthsAndExit(t) }

func TestLoopPinScalarExitWithEagerConvergence(t *testing.T) {
	saved := callFreeLoopRegionEnabled
	callFreeLoopRegionEnabled = false
	defer func() { callFreeLoopRegionEnabled = saved }()
	testLoopPinScalarWidthsAndExit(t)
}

func testLoopPinScalarWidthsAndExit(t *testing.T) {
	for _, pair := range [][2]wasm.ValType{
		{wasm.I32, wasm.I32}, {wasm.I64, wasm.I64}, {wasm.F32, wasm.F32}, {wasm.F64, wasm.F64},
		{wasm.I32, wasm.I64}, {wasm.I64, wasm.I32}, {wasm.F32, wasm.F64}, {wasm.F64, wasm.F32},
	} {
		typ, loopType := pair[0], pair[1]
		m := loopPinScalarModule(t, typ, loopType)
		word := func(v float64) uint64 {
			switch typ {
			case wasm.F32:
				return uint64(math.Float32bits(float32(v)))
			case wasm.F64:
				return math.Float64bits(v)
			}
			return uint64(v)
		}
		for _, enabled := range []bool{false, true} {
			for _, n := range []uint64{0, 1, 2, 5} {
				args := []uint64{n}
				for i := 0; i < 30; i++ {
					value := word(1)
					if typ == wasm.I64 && i == 0 {
						value |= uint64(1) << 63
					}
					args = append(args, value)
				}
				want := word(math.Pow(3, float64(n)) + 241)
				if typ == wasm.I64 {
					want += uint64(1) << 63
				}
				var stats ModuleStats
				var statsArg *ModuleStats
				if diagnosticsEnabled {
					statsArg = &stats
				}
				opts := CompileOptions{Stats: statsArg, Optimizations: map[string]bool{"inline": false, "loop-pin-exchange": enabled}}
				got, err := runArm64WrapperWithOptions(t, m, opts, args...)
				if err != nil {
					t.Fatal(err)
				}
				if typ == wasm.I32 || typ == wasm.F32 {
					got = uint64(uint32(got))
				}
				if got != want {
					t.Fatalf("type=%v enabled=%v n=%d got=%x want=%x", typ, enabled, n, got, want)
				}
				if diagnosticsEnabled && enabled && stats.Funcs[0].Peephole["loop-pin-exchange"] == 0 {
					t.Fatalf("type=%v did not promote a loop local: %v", typ, stats.Funcs[0].Peephole)
				}
			}
		}
	}
}

// A higher-scoring floating-point local cannot borrow an integer register or
// prevent a lower-scoring integer local from receiving an available lease.
func TestLoopPinPlanKeepsRegisterBanks(t *testing.T) {
	f := fn{nLocals: 3, usesCalls: true, localType: []machineType{mtI32, mtF32, mtI64},
		locals:          []localDef{{reg: X19, state: lsStackReg}, {reg: regNone, state: lsMem}, {reg: regNone, state: lsMem}},
		pinnedLocalMask: maskOf(X19), classifier: wasm.NewModuleInstructionClassifier(nil, false)}
	body := []byte{0x20, 1, 0x1a, 0x20, 1, 0x1a, 0x20, 1, 0x1a, 0x20, 1, 0x1a, 0x20, 1, 0x1a,
		0x20, 2, 0x20, 2, 0x7c, 0x20, 2, 0x7c, 0x21, 2, 0x41, 1, 0x0d, 0, 0x0b}
	plan, _ := f.planLoopPinExchange(wasm.NewReader(body))
	if want := loopPinEntry(0, 2, X19); plan != want {
		t.Fatalf("plan=%x want=%x", plan, want)
	}
	f.locals[2].state = lsConstZero
	if plan, _ := f.planLoopPinExchange(wasm.NewReader(body)); plan != 0 {
		t.Fatal("leased an unmaterialized zero or the wrong register bank")
	}
}

func TestLoopPinInitialI32Parameter(t *testing.T) {
	params := []wasm.ValType{wasm.I32}
	for range 30 {
		params = append(params, wasm.I64)
	}
	params = append(params, wasm.I32)
	const counter, acc, result = 32, 33, 34
	body := []byte{2, 1, 0x7f, 2, 0x7e, 0x41, 1, 0x21, counter, 3, 0x40}
	first := true
	for x := byte(1); x <= 30; x++ {
		for range 8 {
			body = append(body, 0x20, x)
			if !first {
				body = append(body, 0x7c)
			}
			first = false
		}
	}
	body = append(body, 0x21, acc, 0x20, counter, 0x41, 1, 0x6b, 0x22, counter, 0x0d, 0, 0x0b,
		0x20, 0, 0x21, counter, 0x02, 0x40, 0x20, counter, 0x45, 0x0d, 0, 3, 0x40,
		0x20, 31, 0xad, 0x20, 31, 0xad, 0x7c, 0x20, 31, 0xad, 0x7c, 0x21, result,
		0x20, counter, 0x41, 1, 0x6b, 0x22, counter, 0x0d, 0, 0x0b, 0x0b,
		0x20, result, 0x20, acc, 0x7c, 0x20, 1, 0x7c, 0x10, 1, 0x0b)
	m := modFuncs(t, funcDef{params, []wasm.ValType{wasm.I64}, body}, funcDef{[]wasm.ValType{wasm.I64}, []wasm.ValType{wasm.I64}, []byte{0, 0x20, 0, 0x0b}})
	const input = uint64(0xface00008000000f)
	for _, enabled := range []bool{false, true} {
		for _, n := range []uint64{0, 1, 7} {
			args := []uint64{n, uint64(1)<<63 | 1}
			for range 29 {
				args = append(args, 1)
			}
			args = append(args, input)
			var stats ModuleStats
			var statsArg *ModuleStats
			if diagnosticsEnabled {
				statsArg = &stats
			}
			opts := CompileOptions{Stats: statsArg, Optimizations: map[string]bool{"inline": false, "loop-pin-exchange": enabled}}
			got, err := runArm64WrapperWithOptions(t, m, opts, args...)
			if err != nil {
				t.Fatal(err)
			}
			want := uint64(1)<<63 | 241
			if n != 0 {
				want += 3 * uint64(uint32(args[len(args)-1]))
			}
			if got != want {
				t.Fatalf("enabled=%v n=%d got=%x want=%x", enabled, n, got, want)
			}
			if diagnosticsEnabled && enabled && stats.Funcs[0].Peephole["loop-pin-exchange"] == 0 {
				t.Fatal("initial i32 parameter was not promoted")
			}
		}
	}
}
