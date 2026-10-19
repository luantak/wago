//go:build amd64

package amd64

import "os"

var leaDoubleEnabled = os.Getenv("WAGO_AMD64_LEA_DOUBLE") == "1"

// A borrowed source cannot be shifted destructively. Doubling it with LEA
// avoids the separate source copy, while an owned or in-place source keeps SHL.
func (f *fn) tryLeaDouble(node *elem, dest Reg) Reg {
	if !f.opt(optLeaDouble) || node.deferredOp() != opShl {
		return regNone
	}
	left, right := node.arg0, node.arg1
	if !left.isValue() || (left.st.kind != stLocalReg && left.st.kind != stGlobReg) ||
		!right.isValue() || right.st.kind != stConst || left.st.reg == RSP || left.st.reg == dest {
		return regNone
	}
	mask := int64(31)
	if node.valueType() == mtI64 {
		mask = 63
	}
	if right.st.cval&mask != 1 {
		return regNone
	}
	src := left.st.reg
	if dest == regNone {
		old := f.pinned
		f.pinned = f.pinned.add(src)
		dest = f.allocReg(maskOf(src))
		f.pinned = old
	}
	f.a.LeaScaledW(dest, src, src, 0, 0, node.valueType().is64())
	f.consumeBlockBelow(node)
	f.occupy(node, dest)
	node.setDeferredOp(opNone)
	f.stats.peep("lea-double")
	return dest
}
