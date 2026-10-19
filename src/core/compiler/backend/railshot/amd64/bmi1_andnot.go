//go:build amd64

package amd64

import (
	"os"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
)

var bmi1AndNotEnabled = os.Getenv("WAGO_AMD64_BMI1_ANDNOT") == "1"

func (f *fn) tryBMI1Logic(node *elem, dest Reg) Reg {
	if node.deferredOp() != opAnd {
		return regNone
	}
	if f.opt(optBMI1ResetLowestBit) {
		if r := f.tryBMI1ResetLowestBit(node, dest); r != regNone {
			return r
		}
	}
	if !f.opt(optBMI1AndNot) {
		return regNone
	}
	typ := node.valueType()
	if typ != mtI32 && typ != mtI64 {
		return regNone
	}
	// Admit only already-evaluated values and current local reads. Do not move
	// a deferred memory access or trapping subtree across the other operand.
	leaf := func(e *elem) bool {
		return e != nil && e.isValue() && e.st.typ == typ && (e.st.kind == stReg || e.st.kind == stSlot || e.st.kind == stLocalReg || e.st.kind == stLocalRef)
	}
	for _, pair := range [][2]*elem{{node.arg0, node.arg1}, {node.arg1, node.arg0}} {
		inv, other := pair[0], pair[1]
		if inv == nil || !inv.isDeferred() || inv.deferredOp() != opXor || !leaf(other) {
			continue
		}
		x, c := inv.arg0, inv.arg1
		if x != nil && x.isValue() && x.st.kind == stConst {
			x, c = c, x
		}
		if !leaf(x) || c == nil || !c.isValue() || c.st.kind != stConst || c.st.typ != typ {
			continue
		}
		if typ == mtI32 && uint32(c.st.cval) != ^uint32(0) || typ == mtI64 && c.st.cval != -1 {
			continue
		}
		if !f.cpuHas(shared.AMD64BMI1) {
			return regNone
		}
		saved := f.pinned
		rx, ownX := f.materializeRead(x)
		f.pinned = f.pinned.add(rx)
		ry, ownY := f.materializeRead(other)
		f.pinned = f.pinned.add(ry)
		rd := dest
		if rd == regNone {
			if ownX {
				rd = rx
			} else if ownY {
				rd = ry
			} else {
				rd = f.allocReg(maskOf(rx, ry))
			}
		}
		f.a.AndNot(rd, rx, ry, typ == mtI64)
		f.pinned = saved
		if ownX && rx != rd {
			f.release(rx)
		}
		if ownY && ry != rd && ry != rx {
			f.release(ry)
		}
		f.consumeBlockBelow(node)
		f.occupy(node, rd)
		node.setDeferredOp(opNone)
		f.stats.peep("bmi1-andnot")
		return rd
	}
	return regNone
}
