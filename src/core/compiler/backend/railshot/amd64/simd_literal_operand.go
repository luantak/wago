//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var simdLiteralOperandEnabled = os.Getenv("WAGO_AMD64_SIMD_LITERAL_OPERAND") == "1"

// Only direct integer binary instructions qualify. No operand is reordered,
// and no guest-memory access is folded or suppressed. In particular, swizzle,
// shifts, floating-point min/max and reversed comparisons use normal lowering.
func simdLiteralBinary(sub uint32) simdBinaryOp {
	switch sub {
	case 78:
		return opVPand
	case 80:
		return opVPor
	case 81:
		return opVPxor
	case 110:
		return opVPaddb
	case 111:
		return opVPaddsb
	case 112:
		return opVPaddusb
	case 113:
		return opVPsubb
	case 114:
		return opVPsubsb
	case 115:
		return opVPsubusb
	case 118:
		return opVPminsb
	case 119:
		return opVPminub
	case 120:
		return opVPmaxsb
	case 121:
		return opVPmaxub
	case 123:
		return opVPavgb
	case 142:
		return opVPaddw
	case 143:
		return opVPaddsw
	case 144:
		return opVPaddusw
	case 145:
		return opVPsubw
	case 146:
		return opVPsubsw
	case 147:
		return opVPsubusw
	case 149:
		return opVPmullw
	case 150:
		return opVPminsw
	case 151:
		return opVPminuw
	case 152:
		return opVPmaxsw
	case 153:
		return opVPmaxuw
	case 155:
		return opVPavgw
	case 174:
		return opVPaddd
	case 177:
		return opVPsubd
	case 181:
		return opVPmulld
	case 182:
		return opVPminsd
	case 183:
		return opVPminud
	case 184:
		return opVPmaxsd
	case 185:
		return opVPmaxud
	case 186:
		return opVPmaddwd
	case 206:
		return opVPaddq
	case 209:
		return opVPsubq
	}
	return 0
}

func (f *fn) trySIMDLiteralOperand(r *wasm.Reader, lo, hi uint64) bool {
	if !f.opt(optSIMDLiteralOperand) || lo|hi == 0 || f.depth() == 0 || f.moduleEH || len(f.customInstructions) != 0 {
		return false
	}
	scan := *r
	prefix, err := scan.Byte()
	if err != nil || prefix != 0xfd {
		return false
	}
	sub, err := scan.U32()
	if err != nil {
		return false
	}
	op := simdLiteralBinary(sub)
	if op == 0 {
		return false
	}
	feature := shared.AMD64Features(op>>12) & (shared.AMD64SSSE3 | shared.AMD64SSE41 | shared.AMD64SSE42)
	if !f.cpuHas(shared.AMD64AVX | feature) {
		return false
	}
	if f.s.back().st.typ != mtV128 {
		return false
	}
	a := f.popValue()
	left, owned := f.operandRegV128(a)
	saved := f.fpinned
	f.fpinned = saved.add(left)
	dst := left
	if !owned {
		dst = f.allocFReg(maskOf(left))
	}
	if literal, ok := f.v128ConstCached(lo, hi); ok {
		op.emit(f, dst, left, literal)
	} else {
		m := byte(0)
		if feature != 0 {
			m = 0x38
		}
		site := f.a.VexMapRipPlaceholder(m, 1, byte(op), dst, left)
		f.recordV128Const(lo, hi, site)
	}
	f.fpinned = saved
	f.pushVReg(dst)
	*r = scan
	f.stats.peep("simd-literal-operand")
	return true
}
