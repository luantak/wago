//go:build (linux || darwin) && amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	coreruntime "github.com/wago-org/wago/src/core/runtime"
)

// Windows commits the full reservation. Two 4 GiB memories exceed typical CI
// commit limits, so run this sparse boundary test on hosts with lazy reservation.
func TestDirectMap32FullMemoryWrapAndOffsets(t *testing.T) {
	for _, fp := range []bool{false, true} {
		for _, offset := range []byte{0, 4} {
			for _, storeOffset := range []bool{false, true} {
				t.Run(fmt.Sprintf("fp=%t/offset=%d/store=%t", fp, offset, storeOffset), func(t *testing.T) {
					source := directMap32Fixture(t, fp, true)
					candidate := directMap32Fixture(t, fp, false)
					load, store := byte(0x28), byte(0x36)
					if fp {
						load, store = 0x2a, 0x38
					}
					for _, m := range []*wasm.Module{source, candidate} {
						m.Memories[0].Limits.Min = 65536
						op := load
						if storeOffset {
							op = store
						}
						at := bytes.Index(m.Code[0].BodyBytes, []byte{op, 0, 0})
						if at < 0 {
							t.Fatal("fixture memory instruction missing")
						}
						m.Code[0].BodyBytes[at+2] = offset
					}
					opts := CompileOptions{AMD64FeaturesSet: true, AMD64Features: shared.AMD64ModernBaseline}
					a := newDirectMap32Run(t, source, opts, 1<<32, 0)
					b := newDirectMap32Run(t, candidate, opts, 1<<32, 0)
					for _, pair := range [][2]uint64{{256, 0xfffffff0}, {0xfffffff0, 256}, {0xfffffff0, 0xfffffff0}, {0xfffffff1, 256}} {
						for _, r := range []*directMap32Run{a, b} {
							mem := r.jm.CurrentBytes()
							clear(mem[:512])
							clear(mem[0xffffffe0:])
							for i := 0; i < 64; i++ {
								binary.LittleEndian.PutUint32(mem[256+i*4:], uint32(i)+0x3f800001)
							}
							for i := 0; i < 8; i++ {
								binary.LittleEndian.PutUint32(mem[0xffffffe0+i*4:], uint32(i)+0x3f800001)
							}
						}
						scale, bias := uint64(0xffffffff), uint64(0x80000001)
						if fp {
							scale, bias = f32b(1.0000001), f32b(-1)
						}
						ae, be := a.call(pair[0], pair[1], 0, 17, scale, bias), b.call(pair[0], pair[1], 0, 17, scale, bias)
						am, bm := a.jm.CurrentBytes(), b.jm.CurrentBytes()
						if (ae == nil) != (be == nil) || !bytes.Equal(am[:512], bm[:512]) || !bytes.Equal(am[0xffffffe0:], bm[0xffffffe0:]) || ae == nil && !bytes.Equal(a.out, b.out) {
							t.Fatal(fp, offset, storeOffset, pair, ae, be)
						}
						if ae != nil {
							x := ae.(*coreruntime.TrapError)
							y := be.(*coreruntime.TrapError)
							if x.Code != y.Code {
								t.Fatal(x, y)
							}
						}
					}
				})
			}
		}
	}
}
