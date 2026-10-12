//go:build arm64

package arm64

import (
	"encoding/binary"
	"testing"
)

func TestMixedBranchAndStoreLoadFolds(t *testing.T) {
	// The last branch and load have external entries and must remain intact.
	words := []uint32{
		0x54000041, 0x14000005, // B.NE +8; B 24
		0xF90003E0, 0xF94003E1, // STR X0,[SP]; LDR X1,[SP]
		0xB9000BE2, 0xB9400BE2, // STR W2,[SP,#8]; LDR W2,[SP,#8]
		0xD65F03C0,
		0x54000040, 0x17FFFFF8, // B.EQ +8; B 0 (external entry at 32)
		0xF9000BE3, 0xF9400BE4, // STR X3,[SP,#16]; LDR X4,[SP,#16] (entry at 40)
		0xD65F03C0,
	}
	for _, branches := range []bool{false, true} {
		for _, stores := range []bool{false, true} {
			b := make([]byte, 4*len(words))
			for i, w := range words {
				binary.LittleEndian.PutUint32(b[4*i:], w)
			}
			targets := make([]uint64, 1)
			for _, pc := range []int{0, 24, 32, 40} {
				branchTargetAdd(targets, pc, len(b))
			}
			f := fn{sc: &scratch{}}
			f.foldBranchPairsAndStoreLoads(b, len(b), targets, branches, stores)
			for i, want := range words {
				if branches {
					switch i {
					case 0:
						want = 0x540000C0
					case 1:
						want = nopWord
					}
				}
				if stores {
					switch i {
					case 3:
						want = 0xAA0003E1
					case 5:
						want = nopWord
					}
				}
				if got := rdWord(b, 4*i); got != want {
					t.Fatalf("branches=%v stores=%v pc=%d: got=%08x want=%08x", branches, stores, 4*i, got, want)
				}
			}
		}
	}
}
