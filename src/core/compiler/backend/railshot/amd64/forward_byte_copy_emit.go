//go:build amd64

package amd64

import "github.com/wago-org/wago/src/core/compiler/wasm"

// Version complete affine forward-copy loops. Every guard runs before guest
// writes; rejection enters the unchanged scalar loop and preserves partial
// writes on traps. REP MOVSB retains the original forward byte order, including
// overlapping copies whose output feeds subsequent source reads.
func (f *fn) emitForwardByteCopy(r *wasm.Reader) {
	if !f.opt(optForwardByteCopy) || f.integerMemory != nil || f.pureReduce != nil || f.interruptible ||
		f.moduleEH || f.gcFrameRoots != nil || f.localBase != 0 || len(f.customInstructions) != 0 ||
		f.depth() != 0 || f.pinRelinquished || f.intervalActive != 0 || f.threadedMemory0 ||
		f.m.MemCount() == 0 || f.memoryAddr64(0) {
		return
	}
	p, ok := f.inspectForwardByteCopy(r)
	if !ok {
		return
	}
	fixed := maskOf(RAX, RDX, RCX, RSI, RDI, R8)
	if f.pinned.union(f.pinnedLocalMask).union(f.reserved)&fixed != 0 {
		return
	}
	for _, reg := range [...]Reg{RAX, RDX, RCX, RSI, RDI, R8} {
		if f.regUser[reg] != nil {
			return
		}
	}
	oldPinned := f.pinned
	f.pinned = f.pinned.union(fixed)
	f.pureReduceFrameUsed = true
	slot := f.maxSpill
	f.maxSpill += p.n + 2
	off := func(i int) int32 { return f.spillOff(slot + i) }
	srcOff, dstOff := off(p.n), off(p.n+1)
	// Snapshot every initial value before changing any pin. The final local
	// assignments can depend on each other through the initial iteration state.
	for i := 0; i < p.n; i++ {
		x := p.local[i]
		if reg, _, pin := f.pinReg(x); pin {
			f.recoverLocal(x)
			f.moveInt(RAX, reg, mtI32)
		} else if f.locals[x].state == lsConstZero {
			f.a.XorSelf32(RAX)
			if p.written&(1<<i) != 0 {
				f.storeFrameInt(f.localAddr(x), RAX, mtI32)
				f.locals[x].state = lsMem
			}
		} else {
			f.loadFrameInt(RAX, f.localAddr(x), mtI32)
		}
		f.a.Store64(RSP, off(i), RAX)
	}
	var failures [40]int
	failN := 0
	fail := func(cc Cond) { failures[failN] = f.a.JccPlaceholder(cc); failN++ }
	k := int32(p.copies)
	f.a.Load32(RAX, RSP, off(p.counter))
	f.a.AluRI(cmpDigit, RAX, k, false)
	fail(condB)
	f.a.MovImm32(RCX, k)
	f.a.XorSelf32(RDX)
	f.a.Div(RCX, false)
	if p.exact {
		f.a.TestSelf(RDX, false)
		fail(condNE)
	}
	f.a.MovRegReg32(R8, RAX)
	f.a.AluRI(5, R8, 1, false) // last iteration index, preserved across REP
	f.a.ImulRI(RAX, k, false)
	f.a.MovRegReg32(RCX, RAX)
	// Amortize the range proofs and string-engine startup over a useful span.
	f.a.AluRI(cmpDigit, RCX, 64, false)
	fail(condB)
	affine := func(a byteCopyAffine) {
		f.a.MovImm32(RAX, int32(a.constant))
		for i, c := range a.coeff {
			if c == 0 {
				continue
			}
			f.a.Load32(RDX, RSP, off(i))
			if c != 1 {
				f.a.ImulRI(RDX, int32(c), false)
			}
			f.a.Add32(RAX, RDX)
		}
	}
	address := func(a byteCopyAccess) {
		affine(a.address)
		if a.offset != 0 {
			f.a.MovImm64(RDX, a.offset)
			f.a.Add64(RAX, RDX)
		}
	}
	for stream, accesses := range [][16]byteCopyAccess{p.src, p.dst} {
		baseOff := srcOff
		if stream != 0 {
			baseOff = dstOff
		}
		address(accesses[0])
		f.a.Store64(RSP, baseOff, RAX)
		f.a.MovReg64(RDX, RAX)
		f.a.Add64(RDX, RCX)
		f.a.MovImm64(RDI, 1<<32)
		f.a.Cmp64(RDX, RDI)
		fail(condA)
		f.a.AluRM(cmpRMcode, RDX, RBX, -bdCurBytes, true)
		fail(condA)
		// Separate Wasm memory immediates do not wrap. Check native address
		// equalities, rather than inferring them from modular affine equality.
		for i := 1; i < p.copies; i++ {
			address(accesses[i])
			f.a.Load64(RDX, RSP, baseOff)
			f.a.AluRI(0, RDX, int32(i), true)
			f.a.Cmp64(RAX, RDX)
			fail(condNE)
		}
	}
	f.a.Load64(RSI, RSP, srcOff)
	f.a.Load64(RDI, RSP, dstOff)
	f.a.Add64(RSI, RBX)
	f.a.Add64(RDI, RBX)
	f.a.RepMovsb()
	// Advance the saved independent induction variables to the beginning of
	// the last scalar iteration, then evaluate its final assignments exactly.
	for i := 0; i < p.n; i++ {
		if p.step[i] == 0 {
			continue
		}
		f.a.MovRegReg32(RAX, R8)
		f.a.ImulRI(RAX, int32(p.step[i]), false)
		f.a.Load32(RDX, RSP, off(i))
		f.a.Add32(RAX, RDX)
		f.a.Store64(RSP, off(i), RAX)
	}
	for i := 0; i < p.n; i++ {
		if p.written&(1<<i) == 0 {
			continue
		}
		affine(p.value[i])
		x := p.local[i]
		if reg, _, pin := f.pinReg(x); pin {
			f.moveInt(reg, RAX, mtI32)
		}
		f.storeFrameInt(f.localAddr(x), RAX, mtI32)
	}
	finished := f.a.JmpPlaceholder()
	for _, site := range failures[:failN] {
		f.a.PatchRel32(site, f.a.Len())
	}
	if forwardByteCopyAssertFast {
		f.trapAlways(trapUnreachable)
	}
	f.pinned = oldPinned
	for i := 0; i < p.n; i++ {
		if p.written&(1<<i) == 0 {
			continue
		}
		x := p.local[i]
		if _, _, pin := f.pinReg(x); pin {
			f.locals[x].state = lsReg
		} else {
			f.locals[x].state = lsMem
		}
	}
	f.integerMemory = &pureReduceJoin{depth: len(f.ctrl) + 1, site: finished}
	f.invalidateBoundsCert()
	f.invalidateStoreForward()
	f.stats.peep("forward-byte-copy")
}
