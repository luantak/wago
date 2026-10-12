//go:build arm64

package arm64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"math/bits"
)

func (p *regionLoopPlan) map32Int(id uint8) bool {
	n := p.nodes[id]
	return n.op == 0 || n.op == 0x28 || n.op == 0x6a || n.op == 0x6c
}

func (p *regionLoopPlan) map32Float(id uint8) bool {
	n := p.nodes[id]
	return n.op == 0x43 || n.op == 0x2a || n.op == 0x92 || n.op == 0x94 || n.op == 0x20 && p.locals[n.bits].typ == mtF32
}

func (p *regionLoopPlan) qualifyMap32() bool {
	if p.zeroTerminated {
		if p.locals[p.counter].step != 0xffffffff || p.nodes[p.limit].bits != 0 || p.nodes[p.limit].uses != 0 || p.headerCount != 0 && p.headerCount != p.counter+1 {
			return false
		}
	} else {
		if p.headerCount != 0 || p.nodes[p.limit].op != 0 || p.nodes[p.limit].bits != 0 {
			return false
		}
		limitInputs := 0
		for _, c := range p.nodes[p.limit].coeff {
			if c != 0 {
				if c != 1 {
					return false
				}
				limitInputs++
			}
		}
		if limitInputs != 1 {
			return false
		}
	}
	if p.storeN != 1 || p.loadN != 1 || p.loads[0].bytes != 4 || p.loads[0].stride != 4 || p.stride(p.stores[0].address) != 4 {
		return false
	}
	value := p.stores[0].value
	add := p.nodes[value]
	mulID := add.left
	if add.op == 0x94 || add.op == 0x6c {
		mulID = value
	}
	mul := p.nodes[mulID]
	fp := p.nodes[p.loads[0].node].op == 0x2a
	if fp {
		if (add.op != 0x92 && add.op != 0x94) || mul.op != 0x94 {
			return false
		}
	} else if (add.op != 0x6a && add.op != 0x6c) || mul.op != 0x6c {
		return false
	}
	if mul.left != p.loads[0].node || !p.map32Invariant(mul.right, fp) || mulID != value && !p.map32Invariant(add.right, fp) {
		return false
	}
	for i, l := range p.locals[:p.localN] {
		if !l.written {
			continue
		}
		if l.step == 0 && (l.value != value || p.inputMask&(1<<i) != 0) {
			return false
		}
	}
	var events [4]uint8
	n := 0
	for _, ev := range p.events[:p.eventN] {
		if ev&0x80 == 0 && p.nodes[ev].op == 0x43 {
			continue
		}
		if n == len(events) {
			return false
		}
		events[n] = ev
		n++
	}
	if mulID == value {
		return n == 3 && events == [4]uint8{p.loads[0].node, value, 0x80, 0}
	}
	return n == 4 && events == [4]uint8{p.loads[0].node, mulID, value, 0x80}
}

func (p *regionLoopPlan) map32Invariant(id uint8, fp bool) bool {
	n := p.nodes[id]
	if fp {
		if n.op == 0x43 {
			return true
		}
		if n.op != 0x20 || p.locals[n.bits].typ != mtF32 {
			return false
		}
	} else if n.op != 0 {
		return false
	}
	for i, l := range p.locals[:p.localN] {
		if n.uses&(1<<i) != 0 && l.written {
			return false
		}
	}
	return true
}

const map32MinCount = 16

type regionEntryLocal struct {
	reg   Reg
	state locState
}

type regionLoopEmitter struct {
	f        *fn
	p        *regionLoopPlan
	gp       [4]Reg
	fp       [6]Reg
	slot     int
	saved    [regionLoopMaxLocals]uint8
	savedN   int
	intShift int8 // -1 multiplies; 0 elides; 1..31 shifts.
	fails    [16]int
	failN    int
}

func (e *regionLoopEmitter) off(i int) int32 {
	if i < regionLoopMaxLocals {
		return e.f.spillOff(e.slot + int(e.saved[i]))
	}
	return e.f.spillOff(e.slot + e.savedN + i - regionLoopMaxLocals)
}

func (e *regionLoopEmitter) fail(cond Cond) {
	e.fails[e.failN] = e.f.a.Bcond(cond)
	e.failN++
}

// Qualification excludes input reads of written, non-induction locals.
func (e *regionLoopEmitter) entryLocal(local int, dst Reg) {
	if e.p.locals[local].step != 0 {
		e.f.ld32(dst, SP, e.off(local))
	} else {
		e.loadCurrentLocal(local, dst)
	}
}

func (e *regionLoopEmitter) affine(id uint8, dst, tmp Reg) {
	n := e.p.nodes[id]
	f := e.f
	initialized := uint32(n.bits) != 0
	if initialized {
		f.a.MovImm32(dst, int32(uint32(n.bits)))
	}
	for i, c := range n.coeff {
		if c == 0 {
			continue
		}
		term := tmp
		if !initialized {
			term = dst
		}
		e.entryLocal(i, term)
		if c != 1 {
			f.a.MovImm32(e.gp[2], int32(c))
			f.a.Mul32(term, term, e.gp[2])
		}
		if initialized {
			f.a.Add32(dst, dst, term)
		}
		initialized = true
	}
	if !initialized {
		f.a.MovImm32(dst, 0)
	}
}

func (e *regionLoopEmitter) copyEntryLocals() {
	f := e.f
	for i, l := range e.p.locals[:e.p.localN] {
		// Only evolving induction locals need their entry value saved.
		// Invariant homes survive both the vector body and local commits.
		if l.step == 0 {
			continue
		}
		d := f.locals[l.index]
		switch {
		case d.state == lsConstZero:
			f.a.MovImm64(e.gp[0], 0)
		case d.reg != regNone && d.state != lsMem:
			if d.isFloat {
				f.a.FmovToGpr(e.gp[0], d.reg, false)
			} else {
				f.a.MovReg32(e.gp[0], d.reg)
			}
		default:
			f.ld32(e.gp[0], SP, f.localOff(int(l.index)))
		}
		f.st64(SP, e.off(i), e.gp[0])
	}
}

func (e *regionLoopEmitter) earlyCount() {
	f, p := e.f, e.p
	a, b := e.gp[0], e.gp[1]
	if p.zeroTerminated {
		f.ld32(a, SP, e.off(int(p.counter)))
		f.a.CmpImm32(a, 0)
		e.fail(condE)
	} else {
		e.affine(p.limit, a, e.gp[3])
		f.ld32(b, SP, e.off(int(p.counter)))
		f.a.CmpReg32(a, b)
		e.fail(condBE)
		f.a.Sub32(a, a, b)
		step := p.locals[p.counter].step
		if step != 1 {
			if !f.a.TstImm32(a, step-1) {
				panic("arm64: map count mask is not encodable")
			}
			e.fail(condNE)
			for shift := uint8(0); shift < 32; shift++ {
				if step == 1<<shift {
					f.a.LsrImm32(a, a, shift)
					break
				}
			}
		}
	}
	f.a.CmpImm32(a, map32MinCount)
	e.fail(condB)
	f.st64(SP, e.off(8), a)
}

func (e *regionLoopEmitter) rangeCheck(address uint8, offset uint64, pointerSlot int) {
	f := e.f
	start, count, end, tmp := e.gp[0], e.gp[1], e.gp[2], e.gp[3]
	e.affine(address, start, tmp)
	f.ld32(count, SP, e.off(8))
	// Qualification proves four-byte accesses with a four-byte stride.
	f.a.LslImm(count, count, 2, false)
	f.a.Add64(end, start, count)
	f.a.MovImm64(tmp, 1<<32)
	f.a.CmpReg64(end, tmp)
	e.fail(condA)
	if offset != 0 {
		f.a.MovImm64(tmp, offset)
		f.a.Add64(start, start, tmp)
		f.a.Add64(end, end, tmp)
	}
	f.a.CmpReg64(end, f.memSizeReg)
	e.fail(condA)
	f.st64(SP, e.off(pointerSlot), start)
}

func (e *regionLoopEmitter) aliasGuard() {
	f := e.f
	dst, src, extent, count := e.gp[0], e.gp[1], e.gp[2], e.gp[3]
	f.ld64(dst, SP, e.off(9))
	f.ld64(src, SP, e.off(10))
	f.a.CmpReg64(dst, src)
	equal := f.a.Bcond(condE)
	f.ld32(count, SP, e.off(8))
	f.a.LslImm(extent, count, 2, false)
	f.a.Add64(extent, extent, dst)
	f.a.CmpReg64(extent, src)
	before := f.a.Bcond(condBE)
	f.a.LslImm(extent, count, 2, false)
	f.a.Add64(extent, extent, src)
	f.a.CmpReg64(extent, dst)
	e.fail(condA)
	f.patchBranch19(equal, f.a.Len())
	f.patchBranch19(before, f.a.Len())
}

func (e *regionLoopEmitter) guards() {
	p := e.p
	e.copyEntryLocals()
	e.earlyCount()
	load := p.nodes[p.loads[0].node]
	e.rangeCheck(p.stores[0].address, p.stores[0].offset, 9)
	e.rangeCheck(load.left, load.bits, 10)
	e.aliasGuard()
}

func (e *regionLoopEmitter) broadcast(id uint8, reg Reg) {
	n := e.p.nodes[id]
	if n.op == 0x43 {
		e.f.a.MovImm32(e.gp[3], int32(n.bits))
	} else if n.op == 0x20 {
		d := e.f.locals[e.p.locals[n.bits].index]
		if d.reg != regNone && d.state != lsMem && d.state != lsConstZero {
			e.f.a.NeonDupS(reg, d.reg)
			return
		}
		e.entryLocal(int(n.bits), e.gp[3])
	} else {
		e.affine(id, e.gp[3], e.gp[1])
	}
	e.f.a.NeonDupGprS(reg, e.gp[3])
}

func (e *regionLoopEmitter) commitLocal(local int, value Reg) {
	f := e.f
	l := e.p.locals[local]
	d := f.locals[l.index]
	if d.state == lsConstZero {
		return
	}
	if d.reg == regNone || d.state != lsReg {
		if l.typ == mtF32 {
			f.st32(SP, f.localOff(int(l.index)), value)
		} else {
			f.a.MovReg32(value, value)
			f.st64(SP, f.localOff(int(l.index)), value)
		}
	}
	if d.reg != regNone && d.state != lsMem {
		if l.typ == mtF32 {
			f.a.FmovFromGpr(d.reg, value, false)
		} else {
			f.a.MovReg32(d.reg, value)
		}
	}
}

func (e *regionLoopEmitter) loadCurrentLocal(local int, dst Reg) {
	f := e.f
	l := e.p.locals[local]
	d := f.locals[l.index]
	switch {
	case d.state == lsConstZero:
		f.a.MovImm64(dst, 0)
	case d.reg != regNone && d.state != lsMem:
		if d.isFloat {
			f.a.FmovToGpr(dst, d.reg, false)
		} else {
			f.a.MovReg32(dst, d.reg)
		}
	default:
		f.ld32(dst, SP, f.localOff(int(l.index)))
	}
}

func (e *regionLoopEmitter) vectors(count int, offset int32) {
	if count > 1 && e.fp[3] == e.fp[0] {
		// Extra registers improve scheduling but do not decide admission.
		for i := 0; i < count; i++ {
			e.vectors(1, offset+int32(i*16))
		}
		return
	}
	f, p := e.f, e.p
	recordSources := profileEnabled && f.stats != nil && f.stats.RecordSources
	var previous profileOrigin
	regs := [4]Reg{e.fp[3], e.fp[4], e.fp[5], e.fp[0]}
	if count == 1 {
		regs[0] = e.fp[0]
	}
	add := p.nodes[p.stores[0].value]
	mul := p.nodes[add.left]
	if add.op == 0x94 || add.op == 0x6c {
		mul = add
	}
	if recordSources {
		f.wasmPC = f.tracePCBase + p.nodes[p.loads[0].node].pos
		previous = f.enterProfileInstruction()
	}
	for i := 0; i < count; i++ {
		f.a.LdrQ(regs[i], e.gp[1], offset+int32(i*16))
	}
	if recordSources {
		f.switchProfileOrigin(previous)
		f.wasmPC = f.tracePCBase + mul.pos
		previous = f.enterProfileInstruction()
	}
	if p.nodes[p.loads[0].node].op == 0x2a {
		for i := 0; i < count; i++ {
			f.a.NeonFmul(regs[i], regs[i], e.fp[1], false)
		}
		if recordSources {
			f.switchProfileOrigin(previous)
			f.wasmPC = f.tracePCBase + add.pos
			previous = f.enterProfileInstruction()
		}
		if add.op == 0x92 {
			for i := 0; i < count; i++ {
				f.a.NeonFadd(regs[i], regs[i], e.fp[2], false)
			}
		}
	} else {
		for i := 0; i < count; i++ {
			if e.intShift < 0 {
				f.a.NeonMulS(regs[i], regs[i], e.fp[1])
			} else if e.intShift != 0 {
				f.a.NeonShlS(regs[i], regs[i], uint8(e.intShift))
			}
		}
		if recordSources {
			f.switchProfileOrigin(previous)
			f.wasmPC = f.tracePCBase + add.pos
			previous = f.enterProfileInstruction()
		}
		if add.op == 0x6a {
			for i := 0; i < count; i++ {
				f.a.NeonAddS(regs[i], regs[i], e.fp[2])
			}
		}
	}
	if recordSources {
		f.switchProfileOrigin(previous)
		f.wasmPC = f.tracePCBase + p.stores[0].pos
		previous = f.enterProfileInstruction()
	}
	for i := 0; i < count; i++ {
		f.a.StrQ(e.gp[0], offset+int32(i*16), regs[i])
	}
	if recordSources {
		f.switchProfileOrigin(previous)
	}
}

func (e *regionLoopEmitter) body(checkedTail int) {
	f, p := e.f, e.p
	oldPC := f.wasmPC
	add := p.nodes[p.stores[0].value]
	mul := p.nodes[add.left]
	if add.op == 0x94 || add.op == 0x6c {
		mul = add
	}
	e.intShift = -1
	scale := p.nodes[mul.right]
	if mul.op == 0x6c && scale.op == 0 && scale.uses == 0 {
		v := uint32(scale.bits)
		// i32 multiplication wraps, including a scale of 0x80000000.
		if v != 0 && v&(v-1) == 0 {
			e.intShift = int8(bits.TrailingZeros32(v))
		}
	}
	if e.intShift < 0 {
		e.broadcast(mul.right, e.fp[1])
	}
	if add.op == 0x92 || add.op == 0x6a {
		e.broadcast(add.right, e.fp[2])
	}
	f.ld64(e.gp[0], SP, e.off(9))
	f.ld64(e.gp[1], SP, e.off(10))
	f.a.Add64(e.gp[0], linMemReg, e.gp[0])
	f.a.Add64(e.gp[1], linMemReg, e.gp[1])
	f.ld32(e.gp[2], SP, e.off(8))
	if !f.a.AndImm32(e.gp[2], e.gp[2], ^uint32(3)) {
		panic("arm64: map group mask is not encodable")
	}
	f.st64(SP, e.off(8), e.gp[2])
	f.a.LsrImm32(e.gp[2], e.gp[2], 2)
	// Keep up to seven vectors for the cleanup loop, so the scalar tail
	// still contains at most three elements.
	f.a.AndImm32(e.gp[3], e.gp[2], 7)
	f.a.LsrImm32(e.gp[2], e.gp[2], 3)
	f.a.CmpImm32(e.gp[2], 0)
	noMain := f.a.Bcond(condE)
	top := f.a.Len()
	e.vectors(4, 0)
	e.vectors(4, 64)
	if profileEnabled && f.stats != nil && f.stats.RecordSources {
		f.wasmPC = oldPC
	}
	f.a.AddImm64(e.gp[0], e.gp[0], 128)
	f.a.AddImm64(e.gp[1], e.gp[1], 128)
	f.a.SubsImm32(e.gp[2], e.gp[2], 1)
	more := f.a.Bcond(condNE)
	f.patchBranch19(more, top)
	f.patchBranch19(noMain, f.a.Len())
	f.a.CmpImm32(e.gp[3], 0)
	noCleanup := f.a.Bcond(condE)
	cleanup := f.a.Len()
	e.vectors(1, 0)
	if profileEnabled && f.stats != nil && f.stats.RecordSources {
		f.wasmPC = oldPC
	}
	f.a.AddImm64(e.gp[0], e.gp[0], 16)
	f.a.AddImm64(e.gp[1], e.gp[1], 16)
	f.a.SubsImm32(e.gp[3], e.gp[3], 1)
	more = f.a.Bcond(condNE)
	f.patchBranch19(more, cleanup)
	f.patchBranch19(noCleanup, f.a.Len())
	f.a.NeonUmovS(e.gp[3], e.fp[0], 3)
	for i, l := range p.locals[:p.localN] {
		if !l.written {
			continue
		}
		if l.step != 0 {
			f.ld32(e.gp[0], SP, e.off(i))
			f.ld32(e.gp[1], SP, e.off(8))
			if l.step != 1 {
				f.a.MovImm32(e.gp[2], int32(l.step))
				f.a.Mul32(e.gp[1], e.gp[1], e.gp[2])
			}
			f.a.Add32(e.gp[0], e.gp[0], e.gp[1])
			e.commitLocal(i, e.gp[0])
		} else {
			e.commitLocal(i, e.gp[3])
		}
	}
	e.affine(p.limit, e.gp[0], e.gp[2])
	e.loadCurrentLocal(int(p.counter), e.gp[1])
	f.a.CmpReg32(e.gp[0], e.gp[1])
	tail := f.a.Bcond(condNE)
	f.patchBranch19(tail, checkedTail)
	f.stats.peep("region-loop-map32")
}

func (f *fn) directMap32TailHomes(entry *[256]regionEntryLocal) bool {
	for i := 0; i < f.nLocals; i++ {
		if entry[i].reg != f.locals[i].reg || entry[i].state != f.locals[i].state {
			return false
		}
	}
	return true
}

func (f *fn) tryDirectMap32(r *wasm.Reader) (bool, error) {
	if !f.directMap32 || f.unreachable || f.depth() > 4 || f.moduleEH || f.localBase != 0 || f.interruptible || f.threadedMemory0 || f.gcFrameRoots != nil || len(f.customInstructions) != 0 || f.nLocals > 256 || f.m.MemCount() == 0 || f.memoryAddr64(0) || f.memSizeReg == regNone {
		return false, nil
	}
	look := *r
	bt, err := look.Byte()
	if err != nil || bt != 0x40 {
		return false, nil
	}
	var p regionLoopPlan
	if !inspectRegionLoop(look, f.localType, f.classifier, &p) || !p.qualifyMap32() {
		return false, nil
	}
	prefixDepth := f.depth()
	depth := len(f.ctrl)
	if err := f.opBlock(r, 0x03); err != nil {
		return true, err
	}
	var entry [256]regionEntryLocal
	for i := 0; i < f.nLocals; i++ {
		entry[i] = regionEntryLocal{reg: f.locals[i].reg, state: f.locals[i].state}
	}
	toFast := f.a.Branch()
	fallback := f.a.Len()
	checkedBody := fallback
	f.ctrl[len(f.ctrl)-1].controlSite = checkedBody
	if err := f.bodyLoop(r, depth); err != nil {
		return true, err
	}
	toDone := f.a.Branch()
	valid := !f.unreachable && f.depth() == prefixDepth && f.directMap32TailHomes(&entry)
	e := regionLoopEmitter{f: f, p: &p}
	block := f.pinned.union(f.pinnedLocalMask).union(f.reserved)
	n := 0
	for _, reg := range gpAlloc {
		if !block.has(reg) && f.regUser[reg] == nil {
			e.gp[n] = reg
			n++
			if n == len(e.gp) {
				break
			}
		}
	}
	if n != len(e.gp) {
		valid = false
	}
	mask := f.fpinned.union(f.fpinnedLocalMask).union(f.fconstMask()).union(f.v128ConstMask())
	n = 0
	for reg := Reg(0); reg < 32; reg++ {
		if !mask.has(reg) && f.fregUser[reg] == nil {
			e.fp[n] = reg
			n++
			if n == len(e.fp) {
				break
			}
		}
	}
	if n < 3 {
		valid = false
	} else if n < len(e.fp) {
		for i := 3; i < len(e.fp); i++ {
			e.fp[i] = e.fp[0]
		}
	}
	if !valid {
		f.patchBranch26(toFast, fallback)
		f.patchBranch26(toDone, f.a.Len())
		f.stats.peep("region-loop-exit-reject")
		return true, nil
	}
	f.patchBranch26(toFast, f.a.Len())
	for i, l := range p.locals[:p.localN] {
		if l.step != 0 {
			e.saved[i] = uint8(e.savedN)
			e.savedN++
		}
	}
	e.slot = f.allocSpillSlots(e.savedN + 3)
	oldPinned, oldFPinned := f.pinned, f.fpinned
	for _, reg := range e.gp {
		f.pinned = f.pinned.add(reg)
	}
	for _, reg := range e.fp {
		f.fpinned = f.fpinned.add(reg)
	}
	e.guards()
	for _, site := range e.fails[:e.failN] {
		f.patchBranch19(site, fallback)
	}
	e.body(checkedBody)
	f.pinned, f.fpinned = oldPinned, oldFPinned
	f.patchBranch26(toDone, f.a.Len())
	f.stats.peep("region-loop-fast")
	return true, nil
}
