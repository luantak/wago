//go:build amd64

package amd64

import "os"

var pressureCompactEnabled = os.Getenv("WAGO_AMD64_PRESSURE_COMPACT") == "1"

// Only ordinary integer ALU trees with owned register/constant leaves qualify.
// Finishing one cannot read mutable state, trap, or require fixed registers.
func compactOwnedRegisters(e *elem) int {
	if e == nil || (e.valueType() != mtI32 && e.valueType() != mtI64) {
		return -1
	}
	if e.isValue() {
		switch e.st.kind {
		case stReg:
			return 1
		case stConst:
			return 0
		}
		return -1
	}
	if !e.isDeferred() || !isBinALU(e.deferredOp()) {
		return -1
	}
	a, b := compactOwnedRegisters(e.arg0), compactOwnedRegisters(e.arg1)
	if a < 0 || b < 0 {
		return -1
	}
	return a + b
}

func (f *fn) compactBeforeMemory() {
	if !f.opt(optPressureCompact) || f.depth() < 2 {
		return
	}
	blocked := f.pinned.union(f.pinnedLocalMask).union(f.reserved)
	free := 0
	for _, r := range gpAlloc {
		if !blocked.has(r) && f.regUser[r] == nil {
			free++
		}
	}
	if free > 2 {
		return
	}
	// Never touch the current address expression or any child of another root.
	base := baseOfValentBlock(f.s.head.prev)
	for e := f.s.head.next; e != base; e = e.next {
		if e.st.hasLogicalRoot() && e.isDeferred() && compactOwnedRegisters(e) >= 2 {
			f.condense(e, regNone)
			f.stats.peep("pressure-compact")
			return
		}
	}
}
