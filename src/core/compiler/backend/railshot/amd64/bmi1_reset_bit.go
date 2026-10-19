//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"os"
)

var bmi1ResetLowestBitEnabled = os.Getenv("WAGO_AMD64_BMI1_RESET_LOWEST_BIT") == "1"

// Local expression selection only: both reads must name the same unchanged
// current local. No memory producer, trapping subtree, loop or algorithm is
// recognized or skipped. The original local is not modified by a borrowed read.
func (f *fn) tryBMI1ResetLowestBit(node *elem, dest Reg) Reg {
	if !f.opt(optBMI1ResetLowestBit) || node.deferredOp() != opAnd {
		return regNone
	}
	typ := node.valueType()
	if typ != mtI32 && typ != mtI64 {
		return regNone
	}
	for _, pair := range [][2]*elem{{node.arg0, node.arg1}, {node.arg1, node.arg0}} {
		value, dec := pair[0], pair[1]
		index, ok := localAddressKey(value)
		if !ok || value.st.typ != typ || dec == nil || !dec.isDeferred() {
			continue
		}
		op := dec.deferredOp()
		if op != opSub && op != opAdd {
			continue
		}
		source, constant := dec.arg0, dec.arg1
		if op == opAdd && source != nil && source.isValue() && source.st.kind == stConst {
			source, constant = constant, source
		}
		other, same := localAddressKey(source)
		if !same || other != index || source.st.typ != typ || constant == nil || !constant.isValue() || constant.st.kind != stConst || constant.st.typ != typ {
			continue
		}
		unit := constant.st.cval == 1
		if op == opAdd {
			unit = constant.st.cval == -1
			if typ == mtI32 {
				unit = uint32(constant.st.cval) == ^uint32(0)
			}
		}
		if !unit {
			continue
		}
		if !f.cpuHas(shared.AMD64BMI1) {
			return regNone
		}
		saved := f.pinned
		src, owned := f.materializeRead(value)
		f.pinned = f.pinned.add(src)
		dst := dest
		if dst == regNone {
			if owned {
				dst = src
			} else {
				dst = f.allocReg(maskOf(src))
			}
		}
		f.a.ResetLowestBit(dst, src, typ == mtI64)
		f.pinned = saved
		if owned && src != dst {
			f.release(src)
		}
		f.consumeBlockBelow(node)
		f.occupy(node, dst)
		node.setDeferredOp(opNone)
		f.stats.peep("bmi1-reset-lowest-bit")
		return dst
	}
	return regNone
}
