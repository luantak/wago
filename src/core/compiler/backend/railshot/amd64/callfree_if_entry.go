//go:build amd64

package amd64

// Preserve the actual local homes at an IF split inside a proved call-free
// loop. Neither edge has executed yet, so no slot synchronization is needed:
// existing per-edge reconciliation establishes the later end/exit contract.
// This snapshots only permanent, already resident numeric pins. Dynamic homes,
// pending zeros and GC/exceptional frames retain canonical convergence.
func (f *fn) snapshotCallFreeIfEntry(state *[]locState) bool {
	if *state != nil || !f.usesCalls || f.callFreeLoopDepth == 0 || !f.opt(optLoopRegState) ||
		len(f.pinnedLocals) == 0 || len(f.intervalReg) != 0 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return false
	}
	// Header promotion and IF splitting form one region contract. Without a
	// register-only header, preserving an incidental dirty value at the split
	// can just move its store to a merge, with no backedge traffic removed.
	promoted := false
	for i := len(f.ctrl) - 1; i >= 0; i-- {
		fr := &f.ctrl[i]
		if fr.kind != cfLoop {
			continue
		}
		header := f.frameBranchState(fr)
		if !fr.has(ctrlLoopCallFree) || len(header) != len(f.pinnedLocals) {
			return false
		}
		for _, home := range header {
			promoted = promoted || home == lsReg
		}
		break
	}
	if !promoted {
		return false
	}
	if f.lazyZero {
		for x := range f.locals {
			if f.locals[x].state == lsConstZero {
				return false
			}
		}
	}
	dirty := false
	for _, x := range f.pinnedLocals {
		switch f.locals[x].state {
		case lsReg:
			dirty = true
		case lsStackReg:
		default:
			return false
		}
	}
	if !dirty {
		return false
	}
	t := f.newLocStateBuf()
	for i, x := range f.pinnedLocals {
		t[i] = f.locals[x].state
	}
	*state = t
	f.stats.peep("callfree-if-entry")
	return true
}
