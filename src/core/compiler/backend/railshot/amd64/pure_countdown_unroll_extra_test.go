//go:build linux && amd64

package amd64

import (
	"bytes"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

func TestPureCountdownHintEffects(t *testing.T) {
	m := pureCountdownFixture(t)
	for _, flags := range []funcHintFlags{0, hintHasLoop | hintHasCall, hintHasLoop | hintTouchesMemory, hintHasLoop | hintModuleEH, hintHasLoop | hintHasSIMD} {
		got, n := pureCountdownModuleHinted(m, []funcHints{{flags: flags}})
		if got != m || n != 0 {
			t.Fatalf("effects=%x admitted", flags)
		}
	}
	if _, n := pureCountdownModuleHinted(m, []funcHints{{flags: hintHasLoop}}); n != 1 {
		t.Fatal("pure hint lost rewrite")
	}
}

func TestPureCountdownDoesNotShiftOtherFunctionProofs(t *testing.T) {
	// The store-prefix pass runs before countdown rewriting. Its memory-read
	// proof PCs must remain attached to the untouched function when a second,
	// call/memory-free function is expanded afterwards.
	body := []byte{1, 1, 0x7f, 0x02, 0x40, 0x20, 2, 0x41, 32, 0x46, 0x0d, 0, 0x03, 0x40}
	body = append(body, invariantStoreLoopBody()...)
	body = append(body, 0x0b, 0x20, 3, 0x0b)
	m := modMem(t, 1, []wasm.ValType{wasm.I32, wasm.I32, wasm.I32}, []wasm.ValType{wasm.I32}, body)
	pure := pureCountdownFixture(t)
	m.Code = append(m.Code, pure.Code[0])
	m.FuncTypes = append(m.FuncTypes, m.FuncTypes[0])
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	before0, before1 := append([]byte{}, m.Code[0].BodyBytes...), append([]byte{}, m.Code[1].BodyBytes...)
	old := invariantStoreAssertFast
	invariantStoreAssertFast = true
	defer func() { invariantStoreAssertFast = old }()
	init := func(mem []byte) {
		for i := range mem {
			mem[i] = byte(i*19 + 7)
		}
	}
	for _, workers := range []int{1, 2} {
		var result uint64
		var memory []byte
		for _, on := range []bool{false, true} {
			got, mem, err := runMemAmd64WithOptions(t, m, CompileOptions{Workers: workers, Optimizations: map[string]bool{"loop-invariant-store": true, "pure-countdown-unroll": on}}, init, 512, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !on {
				result, memory = got, append([]byte{}, mem...)
			} else if got != result || !bytes.Equal(mem, memory) {
				t.Fatalf("workers=%d proof isolation failed", workers)
			}
		}
	}
	if !bytes.Equal(before0, m.Code[0].BodyBytes) || !bytes.Equal(before1, m.Code[1].BodyBytes) {
		t.Fatal("source mutated")
	}
}

func TestPureCountdownRejectsUnsafeBodies(t *testing.T) {
	classifier := wasm.NewModuleInstructionClassifier(&wasm.Module{}, true)
	tail := []byte{0x20, 0, 0x41, 0x7f, 0x6a, 0x22, 0, 0x0d, 0, 0x0b}
	for name, prefix := range map[string][]byte{
		"load":              {0x20, 0, 0x28, 2, 0, 0x1a},
		"store":             {0x20, 0, 0x20, 1, 0x36, 2, 0},
		"call":              {0x10, 0},
		"divide":            {0x20, 0, 0x20, 1, 0x6d, 0x1a},
		"global":            {0x23, 0, 0x1a},
		"nested":            {0x02, 0x40, 0x0b},
		"early-branch":      {0x41, 1, 0x0d, 0},
		"counter-overwrite": {0x41, 7, 0x21, 0},
		"i64":               {0x42, 1, 0x1a},
	} {
		body := append(append([]byte{}, prefix...), tail...)
		if _, ok := planPureCountdown(wasm.NewReader(body), classifier, 2); ok {
			t.Fatalf("accepted %s", name)
		}
	}
	for _, limit := range []int{0, 65} {
		if _, ok := planPureCountdown(wasm.NewReader(tail), classifier, limit); ok {
			t.Fatalf("locals=%d accepted", limit)
		}
	}
	long := append(bytes.Repeat([]byte{1}, 257), tail...)
	if _, ok := planPureCountdown(wasm.NewReader(long), classifier, 2); ok {
		t.Fatal("unbounded body accepted")
	}
	bad := append([]byte{}, tail...)
	bad[len(bad)-2] = 1
	if _, ok := planPureCountdown(wasm.NewReader(bad), classifier, 2); ok {
		t.Fatal("escaping branch accepted")
	}
	bad = append([]byte{}, tail...)
	bad[3] = 0x7e
	if _, ok := planPureCountdown(wasm.NewReader(bad), classifier, 2); ok {
		t.Fatal("nonunit decrement accepted")
	}
	if _, ok := planPureCountdown(wasm.NewReader(tail[:len(tail)-1]), classifier, 2); ok {
		t.Fatal("unterminated loop accepted")
	}
}

func TestPureCountdownPrefixAndSubtractLatch(t *testing.T) {
	m := pureCountdownFixture(t)
	code := m.Code[0]
	needle := []byte{0x41, 0x7f, 0x6a, 0x22, 0, 0x0d, 0}
	pos := bytes.Index(code.BodyBytes, needle)
	if pos < 0 {
		t.Fatal("latch missing")
	}
	code.BodyBytes = append([]byte{}, code.BodyBytes...)
	code.BodyBytes[pos+1], code.BodyBytes[pos+2] = 1, 0x6b
	// A live operand prefix must survive both the new private loops and the
	// unchanged source control. Prefix is below the fixture's outer block.
	code.BodyBytes = append([]byte{0x41, 17}, code.BodyBytes...)
	code.BodyBytes = append(code.BodyBytes[:len(code.BodyBytes)-1], 0x73, 0x0b)
	m.Code[0] = code
	if err := wasm.ValidateModule(m); err != nil {
		t.Fatal(err)
	}
	transformed, n := pureCountdownModule(m)
	if n != 1 {
		t.Fatalf("rewrites=%d", n)
	}
	if err := wasm.ValidateModule(transformed); err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 2} {
		a, err := CompileModuleWith(m, CompileOptions{Workers: workers, Optimizations: map[string]bool{"pure-countdown-unroll": false}})
		if err != nil {
			t.Fatal(err)
		}
		b, err := CompileModuleWith(m, CompileOptions{Workers: workers, Optimizations: map[string]bool{"pure-countdown-unroll": true}})
		if err != nil {
			t.Fatal(err)
		}
		for n := uint64(0); n <= 65; n++ {
			args := []uint64{n | 0xdeadbeef00000000, 0x87654321, 7}
			want, got := runCompiledAmd64u(t, a, args...), runCompiledAmd64u(t, b, args...)
			if want != got {
				t.Fatalf("workers=%d n=%d got=%x want=%x", workers, n, got, want)
			}
		}
		a.CodeImage.Close()
		b.CodeImage.Close()
	}
}

func TestPureCountdownExcludedModes(t *testing.T) {
	m := pureCountdownFixture(t)
	for _, mode := range []string{"profile", "interruptible", "compact"} {
		var code [][]byte
		for _, on := range []bool{false, true} {
			opts := CompileOptions{Optimizations: map[string]bool{"pure-countdown-unroll": on}}
			var stats ModuleStats
			switch mode {
			case "profile":
				opts.Profile = true
				opts.Stats = &stats
			case "interruptible":
				opts.Interruptible = true
			case "compact":
				opts.CompactNative = true
			}
			cm, err := CompileModuleWith(m, opts)
			if err != nil {
				t.Fatal(err)
			}
			code = append(code, append([]byte{}, cm.Code...))
			cm.CodeImage.Close()
		}
		if !bytes.Equal(code[0], code[1]) {
			t.Fatalf("%s source changed", mode)
		}
	}
}
