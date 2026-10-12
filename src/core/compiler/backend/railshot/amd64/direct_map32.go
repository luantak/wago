//go:build amd64

package amd64

// These checks use the existing bounded affine plan. Data arithmetic is admitted
// only as one load, one multiply and one add, with invariant scalar operands.
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
	if mul.left != p.loads[0].node || !p.map32Invariant(mul.right, fp) || (mulID != value && !p.map32Invariant(add.right, fp)) {
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
	// Dead data operations can still load or affect floating-point state. Require
	// the exact three data events and the store in their original order.
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

// Qualification excludes input reads of written, non-induction locals.
func (e *regionLoopEmitter) map32Affine(id uint8, dst, tmp Reg) {
	n := e.p.nodes[id]
	initialized := uint32(n.bits) != 0
	if initialized {
		e.f.a.MovImm32(dst, int32(n.bits))
	}
	for i, c := range n.coeff {
		if c == 0 {
			continue
		}
		term := tmp
		if !initialized {
			term = dst
		}
		if e.p.locals[i].step != 0 {
			e.f.a.Load32(term, RSP, e.off(i))
		} else {
			e.map32EntryInteger(i, term)
		}
		if c != 1 {
			e.f.a.ImulRI(term, int32(c), false)
		}
		if initialized {
			e.f.a.Add32(dst, term)
		}
		initialized = true
	}
	if !initialized {
		e.f.a.Xor32(dst, dst)
	}
}

// The qualified streams both advance by four bytes. Equal starts are safe;
// any other overlap must execute the original scalar loop.
func (e *regionLoopEmitter) map32AliasGuards() {
	f := e.f
	dst, extent, src, end := e.gp[0], e.gp[1], e.gp[2], e.gp[3]
	f.a.Load64(dst, RSP, e.off(9+int(e.storeStream[0])))
	f.a.Load64(src, RSP, e.off(9+int(e.loadStream[e.p.loads[0].node])))
	f.a.Cmp64(dst, src)
	equal := f.a.JccPlaceholder(condE)
	f.a.Load32(extent, RSP, e.off(8))
	f.a.ShiftImm(4, extent, 2, true)
	f.a.MovReg64(end, extent)
	f.a.Add64(extent, dst)
	f.a.Cmp64(extent, src)
	before := f.a.JccPlaceholder(condBE)
	f.a.Add64(end, src)
	f.a.Cmp64(end, dst)
	e.fail(condA)
	f.a.PatchRel32(equal, f.a.Len())
	f.a.PatchRel32(before, f.a.Len())
}

func (e *regionLoopEmitter) broadcastMap32(id uint8, reg Reg) {
	n := e.p.nodes[id]
	if n.op == 0x43 {
		e.f.a.MovImm32(e.gp[3], int32(n.bits))
	} else if n.op == 0x20 {
		d := e.f.locals[e.p.locals[n.bits].index]
		if d.state == lsConstZero {
			e.f.a.Xor32(e.gp[3], e.gp[3])
		} else if d.reg != regNone && d.state != lsMem {
			e.f.a.Pshufd(reg, d.reg, 0)
			return
		} else {
			e.f.a.Load32(e.gp[3], RSP, e.f.localAddr(int(e.p.locals[n.bits].index)))
		}
	} else {
		e.affine(id, e.gp[3], e.gp[2])
	}
	e.f.a.MovGprToXmm(reg, e.gp[3], false)
	e.f.a.Pshufd(reg, reg, 0)
}

// Group independent operations when scratch registers are available, while
// retaining admission with the original three-register budget.
func (e *regionLoopEmitter) map32Vectors(count int, offset int32) {
	if count > 1 && e.fp[3] == e.fp[0] {
		for i := 0; i < count; i++ {
			e.map32Vectors(1, offset+int32(i*16))
		}
		return
	}
	f, p := e.f, e.p
	recordSources := profileEnabled && f.stats != nil && f.stats.RecordSources
	regs := [4]Reg{e.fp[3], e.fp[4], e.fp[5], e.fp[0]}
	if count == 1 {
		regs[0] = e.fp[0]
	}
	add := p.nodes[p.stores[0].value]
	mul := p.nodes[add.left]
	if add.op == 0x94 || add.op == 0x6c {
		mul = add
	}
	var previous profileOrigin
	if recordSources {
		f.wasmPC = f.tracePCBase + p.nodes[p.loads[0].node].pos
		previous = f.enterProfileInstruction()
	}
	for i := 0; i < count; i++ {
		f.mov128LoadDisp(regs[i], e.gp[1], offset+int32(i*16))
	}
	if recordSources {
		f.switchProfileOrigin(previous)
		f.wasmPC = f.tracePCBase + mul.pos
		previous = f.enterProfileInstruction()
	}
	if p.nodes[p.loads[0].node].op == 0x2a {
		for i := 0; i < count; i++ {
			f.a.SseRR(0, 0x59, regs[i], e.fp[1], false)
		}
		if recordSources {
			f.switchProfileOrigin(previous)
			f.wasmPC = f.tracePCBase + add.pos
			previous = f.enterProfileInstruction()
		}
		if add.op == 0x92 {
			for i := 0; i < count; i++ {
				f.a.SseRR(0, 0x58, regs[i], e.fp[2], false)
			}
		}
	} else {
		for i := 0; i < count; i++ {
			f.a.SseMapRR(0x66, 0x38, 0x40, regs[i], e.fp[1]) // SSE4.1 PMULLD
		}
		if recordSources {
			f.switchProfileOrigin(previous)
			f.wasmPC = f.tracePCBase + add.pos
			previous = f.enterProfileInstruction()
		}
		if add.op == 0x6a {
			for i := 0; i < count; i++ {
				f.a.SseRR(0x66, 0xfe, regs[i], e.fp[2], false)
			}
		}
	}
	if recordSources {
		f.switchProfileOrigin(previous)
		f.wasmPC = f.tracePCBase + p.stores[0].pos
		previous = f.enterProfileInstruction()
	}
	for i := 0; i < count; i++ {
		f.mov128StoreDisp(e.gp[0], offset+int32(i*16), regs[i])
	}
	if recordSources {
		f.switchProfileOrigin(previous)
	}
}

func (e *regionLoopEmitter) bodyMap32(checkedTail int) {
	f, p := e.f, e.p
	recordSources := profileEnabled && f.stats != nil && f.stats.RecordSources
	oldPC := f.wasmPC
	add := p.nodes[p.stores[0].value]
	mul := p.nodes[add.left]
	if add.op == 0x94 || add.op == 0x6c {
		mul = add
	}
	e.broadcastMap32(mul.right, e.fp[1])
	if add.op == 0x92 || add.op == 0x6a {
		e.broadcastMap32(add.right, e.fp[2])
	}
	f.a.Load64(e.gp[0], RSP, e.off(9+int(e.storeStream[0])))
	f.a.Load64(e.gp[1], RSP, e.off(9+int(e.loadStream[p.loads[0].node])))
	f.a.Add64(e.gp[0], RBX)
	f.a.Add64(e.gp[1], RBX)
	f.a.Load32(e.gp[2], RSP, e.off(8))
	f.a.AluRI(aluTable[opAnd].digit, e.gp[2], -4, false)
	f.a.Store32(RSP, e.off(8), e.gp[2]) // complete scalar iterations
	f.a.ShiftImm(5, e.gp[2], 2, false)
	f.a.MovRegReg32(e.gp[3], e.gp[2])
	f.a.AluRI(aluTable[opAnd].digit, e.gp[3], 3, false)
	f.a.ShiftImm(5, e.gp[2], 2, false)
	// map32MinCount guarantees at least one complete 16-element group.
	top := f.a.Len()
	e.map32Vectors(4, 0)
	if recordSources {
		f.wasmPC = oldPC
	}

	f.a.AluRI(aluTable[opAdd].digit, e.gp[0], 64, true)
	f.a.AluRI(aluTable[opAdd].digit, e.gp[1], 64, true)
	f.a.AluRI(aluTable[opSub].digit, e.gp[2], 1, false)
	f.a.PatchRel32(f.a.JccPlaceholder(condNE), top)
	f.a.TestSelf(e.gp[3], false)
	noCleanup := f.a.JccPlaceholder(condE)
	cleanup := f.a.Len()
	e.map32Vectors(1, 0)
	if recordSources {
		f.wasmPC = oldPC
	}
	f.a.AluRI(aluTable[opAdd].digit, e.gp[0], 16, true)
	f.a.AluRI(aluTable[opAdd].digit, e.gp[1], 16, true)
	f.a.AluRI(aluTable[opSub].digit, e.gp[3], 1, false)
	f.a.PatchRel32(f.a.JccPlaceholder(condNE), cleanup)
	f.a.PatchRel32(noCleanup, f.a.Len())

	// Commit the original last data value and the affine induction locals. The
	// qualified entry/exit homes also form the next scalar remainder's entry.
	f.a.Pshufd(e.fp[0], e.fp[0], 0xff)
	for i, l := range p.locals[:p.localN] {
		if !l.written {
			continue
		}
		if l.step != 0 {
			f.a.Load32(e.gp[0], RSP, e.off(i))
			f.a.Load32(e.gp[1], RSP, e.off(8))
			if l.step != 1 {
				f.a.ImulRI(e.gp[1], int32(l.step), false)
			}
			f.a.Add32(e.gp[0], e.gp[1])
		} else {
			f.a.MovXmmToGpr(e.gp[0], e.fp[0], false)
		}
		d := f.locals[l.index]
		if d.state == lsConstZero {
			continue
		}
		if d.reg == regNone || d.state != lsReg {
			f.a.Store32(RSP, f.localAddr(int(l.index)), e.gp[0])
		}
		if d.reg != regNone && d.state != lsMem {
			if l.typ == mtF32 {
				f.a.MovGprToXmm(d.reg, e.gp[0], false)
			} else {
				f.a.MovRegReg32(d.reg, e.gp[0])
			}
		}
	}
	// Count was rounded down only in the private copy; the original limit and
	// updated counter determine whether the original checked body has a tail.
	e.affine(p.limit, e.gp[0], e.gp[1])
	d := f.locals[p.locals[p.counter].index]
	if d.reg != regNone && d.state != lsMem {
		f.a.MovRegReg32(e.gp[1], d.reg)
	} else {
		f.a.Load32(e.gp[1], RSP, f.localAddr(int(p.locals[p.counter].index)))
	}
	f.a.Cmp32(e.gp[0], e.gp[1])
	f.a.PatchRel32(f.a.JccPlaceholder(condNE), checkedTail)
	f.stats.peep("region-loop-map32")
}

func (e *regionLoopEmitter) map32EntryInteger(i int, reg Reg) {
	f := e.f
	d := f.locals[e.p.locals[i].index]
	if d.state == lsConstZero {
		f.a.Xor32(reg, reg)
	} else if d.reg != regNone && d.state != lsMem {
		f.a.MovRegReg32(reg, d.reg)
	} else {
		f.a.Load32(reg, RSP, f.localAddr(int(e.p.locals[i].index)))
	}
}

// Test the trip count before copying local homes or constructing ranges.
func (e *regionLoopEmitter) map32EarlyCount() {
	f, p := e.f, e.p
	a, b := e.gp[0], e.gp[1]
	if p.zeroTerminated {
		e.map32EntryInteger(int(p.counter), a)
		f.a.TestSelf(a, false)
		e.fail(condE)
	} else {
		for i, c := range p.nodes[p.limit].coeff {
			if c != 0 {
				e.map32EntryInteger(i, a)
			}
		}
		e.map32EntryInteger(int(p.counter), b)
		f.a.Cmp32(a, b)
		e.fail(condBE)
		f.a.Sub32(a, b)
		step := p.locals[p.counter].step
		if step != 1 {
			f.a.TestImm(a, step-1, false)
			e.fail(condNE)
			for shift := uint8(0); shift < 32; shift++ {
				if step == 1<<shift {
					f.a.ShiftImm(5, a, shift, false)
					break
				}
			}
		}
	}
	f.a.AluRI(7, a, map32MinCount, false)
	e.fail(condB)
	f.a.Store32(RSP, e.off(8), a)
}
