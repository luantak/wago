//go:build amd64

package amd64

import "os"

var selectPureGuardEnabled = os.Getenv("WAGO_AMD64_SELECT_PURE_GUARD") == "1"
var cheapSelectGuardEnabled = os.Getenv("WAGO_AMD64_CHEAP_SELECT_GUARD") == "1"
var shortSelectGuardEnabled = os.Getenv("WAGO_AMD64_SHORT_SELECT_GUARD") == "1"
var shortSelectMulEnabled = os.Getenv("WAGO_AMD64_SHORT_SELECT_MUL") == "1"

// Bound both analysis work and register pressure. Only total integer operations
// may be skipped: in particular loads, division, conversions, and calls decline.
// Variable shifts decline because they reclaim RCX even across allocator pins.
func pureSelectCost(e *elem, budget int) (int, bool) {
	if e == nil || budget <= 0 || (e.st.typ != mtI32 && e.st.typ != mtI64) {
		return 0, false
	}
	if e.isValue() {
		switch e.st.kind {
		case stConst, stReg, stSlot, stLocalRef, stLocalReg, stGlobReg:
			return 0, !e.st.hasGCRoot()
		}
		return 0, false
	}
	cost := 1
	switch e.deferredOp() {
	case opMul:
		cost = 4
	case opAdd, opSub, opAnd, opOr, opXor:
	case opShl, opShrS, opShrU, opRotl, opRotr:
		if !e.arg1.isValue() || e.arg1.st.kind != stConst {
			return 0, false
		}
	default:
		return 0, false
	}
	a, ok := pureSelectCost(e.arg0, budget-1)
	if !ok {
		return 0, false
	}
	b, ok := pureSelectCost(e.arg1, budget-1)
	return a + b + cost, ok
}

func (f *fn) tryPureSelectGuard(cond *elem) bool {
	if !f.opt(optSelectPureGuard) || f.usesCalls || len(f.customInstructions) != 0 || len(f.intervalReg) != 0 || f.gcFrameRoots != nil {
		return false
	}
	b := baseOfValentBlock(cond).prev
	if b == f.s.head {
		return false
	}
	a := baseOfValentBlock(b).prev
	if a == f.s.head {
		return false
	}
	work, keep := a, b
	skipCC := condE
	if !work.isDeferred() || !keep.isValue() {
		work, keep = b, a
		skipCC = condNE
	}
	if !work.isDeferred() || !keep.isValue() || rootMachineType(work) != rootMachineType(keep) {
		return false
	}
	cost, ok := pureSelectCost(work, 6)
	minimumCost := 5
	// Cheap additions can use low-latency, flag-preserving address arithmetic;
	// retain the ordinary select rather than introducing a branch for them.
	if f.opt(optCheapSelectGuard) && work.deferredOp() != opAdd {
		minimumCost = 1
	}
	if !ok || cost < minimumCost {
		return false
	}
	if _, ok = pureSelectCost(keep, 1); !ok {
		return false
	}
	// Canonicalize values below the select before the conditional edge. Otherwise
	// a spill emitted only in the work arm could leave the skipped arm stale.
	f.flushBelow(a)
	typ := rootMachineType(keep)
	dst := f.materializeSelectBranch(keep, typ)
	f.pinned = f.pinned.add(dst)
	if f.isFusableTruth(cond) {
		cc := f.condenseToFlags(cond)
		if skipCC == condE {
			cc = invertCond(cc)
		}
		skipCC = cc
	} else {
		cr := f.materialize(cond)
		f.a.TestSelf(cr, false)
		f.release(cr)
		f.erase(cond)
	}
	short := f.opt(optShortSelectGuard) && shortSelectWorkWithMul(work, f.opt(optShortSelectMul))
	skip := 0
	if short {
		skip = f.a.Len() + 1
		f.a.JccRel8(skipCC, f.a.Len()+2)
		// Retain the near-branch instruction footprint so later
		// instructions and branch targets keep their original addresses.
		f.a.B = append(f.a.B, 0x0f, 0x1f, 0x40, 0)
	} else {
		skip = f.a.JccPlaceholder(skipCC)
	}
	src := f.materialize(work)
	f.moveInt(dst, src, typ)
	f.release(src)
	f.erase(work)
	if short {
		if !f.a.PatchRel8(skip, f.a.Len()) {
			panic("amd64: bounded select guard exceeded rel8 range")
		}
		f.stats.peep("short-select-guard")
	} else {
		f.a.PatchRel32(skip, f.a.Len())
	}
	f.pinned = f.pinned.remove(dst)
	f.erase(keep)
	f.pushReg(dst, typ)
	f.invalidateGlobalsCache()
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.stats.peep("select-pure-guard")
	return true
}

// A single scalar ALU operation over already-materialized values has no
// internal control flow. Two operand loads, a result operation/copy, and
// allocator spill stores fit comfortably within the 127-byte forward reach.
// Exclude address expressions, vector values, fixed-register operations, and
// recursive trees so unrelated lowering changes cannot expand this window.
func shortSelectWork(e *elem) bool {
	return shortSelectWorkWithMul(e, false)
}

func shortSelectWorkWithMul(e *elem, mul bool) bool {
	if e == nil || !e.isDeferred() {
		return false
	}
	switch e.deferredOp() {
	case opAdd, opSub, opAnd, opOr, opXor:
	case opMul:
		if !mul {
			return false
		}
	default:
		return false
	}
	for _, arg := range []*elem{e.arg0, e.arg1} {
		if arg == nil || !arg.isValue() || (arg.st.typ != mtI32 && arg.st.typ != mtI64) || arg.st.hasGCRoot() {
			return false
		}
		switch arg.st.kind {
		case stConst, stReg, stSlot, stLocalReg, stLocalRef, stGlobReg:
		default:
			return false
		}
	}
	return true
}
