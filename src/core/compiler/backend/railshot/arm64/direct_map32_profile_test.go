//go:build (linux || darwin || windows) && arm64 && wago_profile

package arm64

import (
	"bytes"
	"testing"
)

func TestDirectMap32OriginalSourcePositionsARM64(t *testing.T) {
	for _, fp := range []bool{false, true} {
		load, mul, add, store := byte(0x28), byte(0x6c), byte(0x6a), byte(0x36)
		if fp {
			load, mul, add, store = 0x2a, 0x94, 0x92, 0x38
		}
		var counts [2][4]int
		for scalar := 0; scalar < 2; scalar++ {
			m := directMap32Fixture(t, fp, scalar != 0)
			old := append([]byte(nil), m.Code[0].BodyBytes...)
			var stats ModuleStats
			cm, err := CompileModuleWith(m, CompileOptions{Stats: &stats, Profile: true, SourceMaps: true})
			if err != nil {
				t.Fatal(err)
			}
			cm.CodeImage.Close()
			if !bytes.Equal(old, m.Code[0].BodyBytes) {
				t.Fatal("source bytecode changed")
			}
			for i, op := range []byte{load, mul, add, store} {
				pc := uint32(bytes.IndexByte(old, op)) + m.Code[0].LocalDeclBytes
				for _, source := range stats.SourceRanges {
					if source.Function == 0 && source.WasmOffset == pc {
						counts[scalar][i]++
					}
				}
			}
		}
		for _, op := range []byte{load, mul, add, store} {
			i := bytes.IndexByte([]byte{load, mul, add, store}, op)
			if counts[0][i] < counts[1][i]+2 {
				t.Fatalf("opcode=%x selected ranges=%d scalar ranges=%d; unrolled and cleanup bodies need their original source", op, counts[0][i], counts[1][i])
			}
		}
	}
}

func TestDirectMap32DoesNotRetainDisabledSourcesARM64(t *testing.T) {
	var stats ModuleStats
	cm, err := CompileModuleWith(directMap32Fixture(t, false, false), CompileOptions{Stats: &stats, Profile: true})
	if err != nil {
		t.Fatal(err)
	}
	defer cm.CodeImage.Close()
	if len(stats.SourceRanges) != 0 {
		t.Fatalf("disabled source maps retained %d ranges", len(stats.SourceRanges))
	}
}
