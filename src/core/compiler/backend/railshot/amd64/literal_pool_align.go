//go:build amd64

package amd64

import (
	"encoding/binary"
	"os"
)

var literalPoolAlignEnabled = os.Getenv("WAGO_AMD64_LITERAL_POOL_ALIGN") == "1"

// Scalar modules keep the original pool emitter untouched. All additional
// vector alignment and ordering work lives on this SIMD-only cold path.
func (f *fn) emitSIMDLiteralPool() {
	if len(f.v128Pool) == 0 || moduleLiteralIsland(f.policy) || !f.alignLiteralPool() {
		f.emitV128ConstPool()
		return
	}
	if diagnosticsEnabled && f.stats != nil {
		for _, c := range f.v128Pool {
			f.stats.literalKeys = append(f.stats.literalKeys, literalKey{lo: c.lo, hi: c.hi, size: c.size})
		}
	}
	start := f.a.Len()
	f.emitAlignedLiteralPool()
	f.v128Pool = f.v128Pool[:0]
	f.poolSites = f.poolSites[:0]
	if diagnosticsEnabled && f.stats != nil {
		f.stats.NativeSize.LiteralPoolBytes += f.a.Len() - start
	}
}

// Align vector-bearing unexecuted tails to their widest literal. Scalar-only
// pools retain their original layout: small unaligned scalar loads do not pay
// the same split-load cost as vectors. Descending-width emission
// then naturally aligns every key without inter-key padding or sorting storage.
// Compact module islands retain their existing layout. Initial padding precedes
// LiteralPoolBytes accounting so it is not reported as duplicate constant data.
func (f *fn) alignLiteralPool() bool {
	// Scalar Wasm modules need no pool walk. Compiler-created vector constants
	// in those modules retain the existing layout as well.
	if !f.moduleHasSIMD || !f.opt(optLiteralPoolAlign) {
		return false
	}
	vector := false
	for _, c := range f.v128Pool {
		if c.size == 16 {
			vector = true
			break
		}
	}
	if !vector {
		return false
	}
	if pad := (-f.a.Len()) & 15; pad != 0 {
		f.a.EmitBytes(alignPad[:pad])
		f.stats.peep("literal-pool-align")
	}
	return true
}

// Keep the uncommon vector ordering loop out of scalar pool emission's hot body.
func (f *fn) emitAlignedLiteralPool() {
	var raw [16]byte
	for width := uint8(16); width >= 4; width >>= 1 {
		for _, c := range f.v128Pool {
			if c.size != width {
				continue
			}
			off := f.a.Len()
			binary.LittleEndian.PutUint64(raw[:8], c.lo)
			binary.LittleEndian.PutUint64(raw[8:], c.hi)
			f.a.EmitBytes(raw[:c.size])
			for head := c.head; head != 0; {
				s := f.poolSites[head-1]
				f.a.PatchRel32(int(s.off), off)
				head = s.next
			}
		}
	}
}
