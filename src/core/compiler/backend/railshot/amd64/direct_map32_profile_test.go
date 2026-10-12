//go:build (linux || darwin || windows) && amd64 && wago_profile

package amd64

import (
	"bytes"
	"testing"
)

func TestDirectMap32OriginalSourcePositions(t *testing.T) {
	for _, fp := range []bool{false, true} {
		m := directMap32Fixture(t, fp, false)
		old := append([]byte(nil), m.Code[0].BodyBytes...)
		var stats ModuleStats
		cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Profile: true, SourceMaps: true})
		if err != nil {
			t.Fatal(err)
		}
		defer cm.CodeImage.Close()
		if !bytes.Equal(old, m.Code[0].BodyBytes) {
			t.Fatal("source bytecode changed")
		}
		load, mul, add, store := byte(0x28), byte(0x6c), byte(0x6a), byte(0x36)
		if fp {
			load, mul, add, store = 0x2a, 0x94, 0x92, 0x38
		}
		for _, op := range []byte{load, mul, add, store} {
			pc := uint32(bytes.IndexByte(old, op)) + m.Code[0].LocalDeclBytes
			n := 0
			for _, s := range stats.SourceRanges {
				if s.Function == 0 && s.WasmOffset == pc {
					n++
				}
			}
			if n < 3 {
				t.Fatalf("opcode=%x original pc=%d source ranges=%d; scalar, unrolled, and cleanup bodies need their original source", op, pc, n)
			}
		}
	}
}
