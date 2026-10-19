//go:build amd64

package amd64

import "os"

var booleanChainOperandsEnabled = os.Getenv("WAGO_AMD64_BOOLEAN_CHAIN_OPERANDS") == "1"

// One result register and one read scratch suffice unless a second read needs
// its own register (a wide immediate or a differently typed frame carrier).
// Check before reserving registers; an owned register spilled by reservation
// becomes a same-width frame operand, which is still covered.
func (f *fn) booleanChainScratchCount(leaves []*elem) int {
	if !f.opt(optBooleanChainOperands) {
		return 3
	}
	for _, p := range leaves {
		if !p.isDeferred() || p.deferredOp() == opEqz {
			continue
		}
		r := p.arg1
		if r.st.typ.is64() != p.valueType().is64() || r.st.kind == stConst && r.st.typ.is64() && !fitsImm32(r.st.cval) {
			return 3
		}
	}
	return 2
}

func (f *fn) booleanChainRight(left Reg, right *elem, wide bool) bool {
	if !f.opt(optBooleanChainOperands) || right.st.typ.is64() != wide {
		return false
	}
	var off int32
	switch right.st.kind {
	case stSlot:
		off = f.spillOff(right.st.slotIndex())
	case stLocalRef:
		off = f.localAddr(right.st.index())
	case stConst:
		if wide && !fitsImm32(right.st.cval) {
			return false
		}
		f.a.AluRI(cmpDigit, left, int32(right.st.cval), wide)
		f.stats.peep("boolean-chain-operands")
		return true
	default:
		return false
	}
	if regallocCheckEnabled {
		f.checkFoldedUse(right)
	}
	f.a.AluRM(cmpRMcode, left, RSP, off, wide)
	f.stats.peep("boolean-chain-operands")
	return true
}

func (f *fn) tryBooleanChainOperands(left, right *elem, wide bool) bool {
	if !f.opt(optBooleanChainOperands) || !booleanReady(left) || !booleanReady(right) || left.st.typ.is64() != wide || right.st.typ.is64() != wide {
		return false
	}
	register := func(e *elem) bool {
		return e.st.kind == stReg || e.st.kind == stLocalReg || e.st.kind == stGlobReg
	}
	frame := func(e *elem) (int32, bool) {
		switch e.st.kind {
		case stSlot:
			return f.spillOff(e.st.slotIndex()), true
		case stLocalRef:
			return f.localAddr(e.st.index()), true
		}
		return 0, false
	}
	// All operands were evaluated before entering the chain. Folding only
	// private frame storage cannot suppress or reorder a guest-memory trap.
	// Do not allocate, release, or rewrite operand ownership on conditional paths.
	check := func(e *elem, folded bool) {
		if regallocCheckEnabled {
			if folded {
				f.checkFoldedUse(e)
			} else {
				f.checkUse(e)
			}
		}
	}
	if register(left) {
		if off, ok := frame(right); ok {
			check(left, false)
			check(right, true)
			f.a.AluRM(cmpRMcode, left.st.reg, RSP, off, wide)
		} else if right.st.kind == stConst && (!wide || fitsImm32(right.st.cval)) {
			check(left, false)
			f.a.AluRI(cmpDigit, left.st.reg, int32(right.st.cval), wide)
		} else {
			return false
		}
	} else if off, ok := frame(left); ok && register(right) {
		check(left, true)
		check(right, false)
		// CMP r/m,reg preserves the original operand order and condition code.
		f.a.AluRM(0x39, right.st.reg, RSP, off, wide)
	} else {
		return false
	}
	f.stats.peep("boolean-chain-operands")
	return true
}
