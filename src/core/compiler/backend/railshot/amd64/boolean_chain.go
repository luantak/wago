//go:build amd64

package amd64

import "os"

var booleanChainEnabled = os.Getenv("WAGO_AMD64_BOOLEAN_CHAIN") == "1"

func booleanReady(e *elem) bool {
	if e == nil || !e.isValue() || e.st.hasGCRoot() || (e.st.typ != mtI32 && e.st.typ != mtI64) {
		return false
	}
	switch e.st.kind {
	case stConst, stReg, stSlot, stLocalRef, stLocalReg, stGlobReg:
		return true
	}
	return false
}

// Plan only total comparisons of already evaluated values. In particular, a
// deferred guest load or division is never moved behind a conditional edge.
func booleanLeaves(e *elem, op wOp, leaves *[16]*elem, n *int, budget *int) bool {
	if e == nil || *budget == 0 {
		return false
	}
	*budget--
	if e.isDeferred() && e.deferredOp() == op {
		return booleanLeaves(e.arg0, op, leaves, n, budget) && booleanLeaves(e.arg1, op, leaves, n, budget)
	}
	if *n == len(leaves) {
		return false
	}
	if e.isDeferred() {
		if e.deferredOp() == opEqz {
			if !booleanReady(e.arg0) {
				return false
			}
		} else if !isCompare(e.deferredOp()) || !booleanReady(e.arg0) || !booleanReady(e.arg1) {
			return false
		}
	} else if !booleanReady(e) || e.st.typ != mtI32 || !(e.st.valueFacts().has(factBoolean) || e.st.kind == stConst && uint64(e.st.cval) <= 1) {
		return false
	}
	leaves[*n] = e
	*n++
	return true
}

func (f *fn) tryBooleanChain(node *elem, dest Reg) (Reg, bool) {
	if !f.opt(optBooleanChain) || (node.deferredOp() != opAnd && node.deferredOp() != opOr) || node.st.typ != mtI32 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return regNone, false
	}
	var leaves [16]*elem
	n, budget := 0, 64
	// Require enough comparisons to amortize the result setup and branch edges.
	if !booleanLeaves(node, node.deferredOp(), &leaves, &n, &budget) || n < 4 {
		return regNone, false
	}
	// Reserve all scratch before branching. Any necessary spills execute on
	// every path; conditional bodies never invoke the allocator or alter homes.
	// Regional local pins remain excluded even in functions using frame registers.
	block := f.pinned.union(f.pinnedLocalMask).union(f.reserved)
	if dest != regNone {
		block = block.add(dest)
	}
	var scratch [3]Reg
	needed := f.booleanChainScratchCount(leaves[:n])
	nr := 0
	for pass := 0; pass < 2; pass++ {
		for _, r := range gpAlloc {
			if block.has(r) {
				continue
			}
			u := f.regUser[r]
			if pass == 0 && u != nil || pass == 1 && (u == nil || !u.isValue() || u.st.kind != stReg || u.st.typ.isXMM()) {
				continue
			}
			scratch[nr] = r
			nr++
			block = block.add(r)
			if nr == needed {
				break
			}
		}
		if nr == needed {
			break
		}
	}
	if nr != needed {
		return regNone, false
	}
	savedPinned := f.pinned
	for _, r := range scratch[:needed] {
		f.spillIfUsed(r)
		f.pinned = f.pinned.add(r)
	}
	result := scratch[0]
	read := func(e *elem, tmp Reg) Reg {
		if regallocCheckEnabled {
			f.checkUse(e)
		}
		switch e.st.kind {
		case stReg, stLocalReg, stGlobReg:
			return e.st.reg
		case stConst:
			f.loadConst(tmp, e.st)
		case stSlot:
			f.loadFrameInt(tmp, f.spillOff(e.st.slotIndex()), e.st.typ)
		case stLocalRef:
			f.loadFrameInt(tmp, f.localAddr(e.st.index()), e.st.typ)
		}
		return tmp
	}
	or := node.deferredOp() == opOr
	initial := int32(0)
	if or {
		initial = 1
	}
	f.a.MovImm32(result, initial)
	var jumps [16]int
	for i, p := range leaves[:n] {
		cc := condNE
		if p.isDeferred() {
			if p.deferredOp() != opEqz && f.tryBooleanChainOperands(p.arg0, p.arg1, p.valueType().is64()) {
				cc = condOf(p.deferredOp())
				if !or {
					cc = invertCond(cc)
				}
				jumps[i] = f.a.JccPlaceholder(cc)
				continue
			}
			left := read(p.arg0, scratch[1])
			wide := p.valueType().is64()
			if p.deferredOp() == opEqz {
				f.a.TestSelf(left, wide)
				cc = condE
			} else {
				if !f.booleanChainRight(left, p.arg1, wide) {
					right := read(p.arg1, scratch[2])
					f.cmpRR(left, right, wide)
				}
				cc = condOf(p.deferredOp())
			}
		} else {
			r := read(p, scratch[1])
			f.a.TestSelf(r, false)
		}
		if !or {
			cc = invertCond(cc)
		}
		jumps[i] = f.a.JccPlaceholder(cc)
	}
	f.a.MovImm32(result, initial^1)
	for _, j := range jumps[:n] {
		f.a.PatchRel32(j, f.a.Len())
	}
	// Release only owned inputs; borrowed local/global pins remain intact.
	if node.deferredDepth() > maxDeferDepth {
		f.stats.peep("boolean-chain-depth")
	}
	base := baseOfValentBlock(node)
	for e := base; e != node; e = e.next {
		if e.isValue() && e.st.kind == stReg {
			f.release(e.st.reg)
		}
	}
	f.consumeBlockBelow(node)
	f.pinned = savedPinned
	if dest != regNone {
		f.moveInt(dest, result, mtI32)
		result = dest
	}
	f.occupy(node, result)
	node.st.typ = mtI32
	node.st.setValueFacts(factBoolean | factUpper32Zero)
	node.setDeferredOp(opNone)
	f.stats.peep("boolean-chain")
	if needed == 2 {
		f.stats.peep("boolean-chain-two-scratch")
	}
	return result, true
}
