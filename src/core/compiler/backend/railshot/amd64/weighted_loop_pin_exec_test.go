//go:build linux && amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"testing"
)

func TestWeightedLoopPinExchangePreservesWrittenDonor(t *testing.T) {
	requireCompilerDiagnostics(t)
	params := make([]wasm.ValType, 12)
	for i := range params {
		params[i] = wasm.I32
	}
	// Whole-function use favors the ten outer locals. The loop needs the
	// otherwise cold final parameter much more often than any donor.
	body := []byte{1, 1, 0x7f}
	for repeat := 0; repeat < 64; repeat++ {
		for local := byte(1); local <= 10; local++ {
			body = append(body, 0x20, 12, 0x20, local, 0x6a, 0x21, 12)
		}
	}
	body = append(body, 0x20, 11, 0x41, 1, 0x6a, 0x21, 11, 0x02, 0x40, 0x20, 0, 0x45, 0x0d, 0, 0x03, 0x40)
	for repeat := 0; repeat < 6; repeat++ {
		body = append(body, 0x20, 12, 0x20, 11, 0x6a, 0x21, 12)
	}
	for local := byte(1); local <= 10; local++ {
		body = append(body, 0x20, local, 0x41, 1, 0x6a, 0x21, local)
	}
	body = append(body, 0x20, 0, 0x41, 1, 0x6b, 0x22, 0, 0x0d, 0, 0x0b, 0x0b, 0x20, 12)
	for local := byte(0); local <= 11; local++ {
		body = append(body, 0x20, local, 0x73)
	}
	body = append(body, 0x0b)
	m := modMem(t, 1, params, []wasm.ValType{wasm.I32}, body)
	for _, n := range []uint64{0, 1, 2, 17} {
		args := []uint64{n, 1, 2, 3, 4, 5, 0xffffffff, 7, 8, 9, 10, 0x87654321}
		var results [2]uint64
		var exchanges [2]int
		for i, enabled := range []bool{false, true} {
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Optimizations: map[string]bool{"weighted-loop-pin-exchange": enabled, "extended-loop-pin-scan": true, "interval-region-pins": false}})
			if err != nil {
				t.Fatal(err)
			}
			results[i] = runCompiledAmd64u(t, cm, args...)
			exchanges[i] = stats.Funcs[0].Peephole["loop-pin-exchange"]
			cm.CodeImage.Close()
		}
		if results[0] != results[1] {
			t.Fatalf("n=%d results=%x", n, results)
		}
		if exchanges[1] <= exchanges[0] {
			t.Fatalf("fixture missed weighted exchange: %v", exchanges)
		}
	}
}
