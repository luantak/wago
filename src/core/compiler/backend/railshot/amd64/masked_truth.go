//go:build amd64

package amd64

import "os"

var maskedTruthEnabled = os.Getenv("WAGO_AMD64_MASKED_TRUTH") == "1"

// Select truth consumers may use flags for a noncanonical masked value. Ordinary
// control edges retain their established staging: broadening those edges did
// not improve all measured control-heavy workloads. Do not widen
// isFusableCompare: value consumers such as store8 must retain every mask bit.
func (f *fn) isMaskedTruth(e *elem) bool {
	return f.opt(optMaskedTruth) && e != nil && e.isDeferred() && e.deferredOp() == opAnd && e.st.typ == mtI32 && e.arg1 != nil && e.arg1.isValue() && e.arg1.st.kind == stConst
}

func (f *fn) isFusableTruth(e *elem) bool {
	return isFusableCompare(e) || f.isMaskedTruth(e)
}

func (f *fn) maskedTruthToFlags(e *elem) Cond {
	left := e.arg0
	var r Reg
	owned := false
	if left.isValue() && (left.st.kind == stLocalReg || left.st.kind == stGlobReg) {
		r = left.st.reg
	} else {
		r = f.materialize(left)
		owned = true
	}
	f.a.TestImm(r, uint32(e.arg1.st.cval), false)
	if owned {
		f.release(r)
	}
	f.consumeBlockBelow(e)
	f.erase(e)
	f.stats.peep("masked-truth")
	return condNE
}
