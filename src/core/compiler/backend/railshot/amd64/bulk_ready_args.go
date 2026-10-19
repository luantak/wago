//go:build amd64

package amd64

import "os"

var bulkReadyArgsEnabled = os.Getenv("WAGO_AMD64_BULK_READY_ARGS") == "1"

// Avoid a canonical-frame round trip when a bulk copy consumes the complete
// stack and its three arguments are already scalar values. Register sources
// move in parallel before any frame or constant loads overwrite destinations.
func (f *fn) prepareReadyCopyArgs() bool {
	if !f.opt(optBulkReadyArgs) || f.depth() != 3 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 || f.guardMode {
		return false
	}
	fixed := maskOf(RDI, RSI, RCX, RDX, R8)
	if f.pinnedLocalMask.union(f.reserved)&fixed != 0 {
		return false
	}
	var args [3]*elem
	e := f.s.head.next
	for i := range args {
		if e == f.s.head || !e.isValue() || e.st.typ != mtI32 || e.st.hasGCRoot() {
			return false
		}
		switch e.st.kind {
		case stConst, stReg, stLocalReg, stGlobReg, stLocalRef, stSlot:
		default:
			return false
		}
		args[i] = e
		e = e.next
	}
	if e != f.s.head {
		return false
	}
	f.invalidateGlobalsCache()
	f.invalidateBoundsCert()
	dsts := [3]Reg{RDI, RSI, RCX}
	var moves [3]regMove
	n := 0
	for i, e := range args {
		switch e.st.kind {
		case stReg, stLocalReg, stGlobReg:
			if regallocCheckEnabled {
				f.checkUse(e)
			}
			moves[n] = regMove{dst: dsts[i], src: e.st.reg}
			n++
		}
	}
	resolveRegMoves(moves[:n], f.a.MovReg64, f.a.Xchg64)
	for i, e := range args {
		switch e.st.kind {
		case stConst:
			f.a.MovImm64(dsts[i], uint64(e.st.cval))
		case stLocalRef:
			if regallocCheckEnabled {
				f.checkFoldedUse(e)
			}
			f.loadFrameInt(dsts[i], f.localAddr(e.st.index()), mtI32)
		case stSlot:
			if regallocCheckEnabled {
				f.checkFoldedUse(e)
			}
			f.a.Load32(dsts[i], RSP, f.spillOff(e.st.slotIndex()))
		}
	}
	// The lowering below owns fixed registers until setDepth removes all three
	// consumed arguments. No allocator action may interpret stale GP owners.
	for _, e := range args {
		if e.st.kind == stReg {
			f.release(e.st.reg)
		}
	}
	f.stats.peep("bulk-ready-args")
	return true
}
