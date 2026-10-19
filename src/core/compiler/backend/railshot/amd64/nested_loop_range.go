//go:build amd64

package amd64

// Seed ordinary cache entries only at control boundaries. Address queries need
// no nested-range test or control-stack walk. Cache eviction merely loses a
// proof: it cannot justify an access outside a statically proved outer range.
func (f *fn) seedEnclosingLoopAddressMaxima() {
	for i := f.loopAddressDepth - 2; i >= 0; i-- {
		if f.ctrl[i].has(ctrlLoopSavedRange) {
			word := f.ctrlMerge(&f.ctrl[i]).loopPinPlan
			f.rememberAddressMaximum(int(uint32(word))-1, word>>32)
		}
	}
}

func (f *fn) activateLoopAddressRange(local uint32, maximum uint64) bool {
	// The planner admits an inner range only with the nested option enabled.
	// Ordinary outer activation needs no repeated policy lookup.
	if f.loopAddressDepth > 0 {
		outer := &f.ctrl[f.loopAddressDepth-1]
		// A nested loop excludes the straight-line pin-exchange grammar. Keep
		// this defensive guard if either grammar is broadened in the future.
		if outer.has(ctrlLoopPinExchange) {
			return false
		}
		m := f.ensureCtrlMerge(outer)
		m.loopPinPlan = uint64(f.loopAddressLocal) | uint64(uint32(f.loopAddressMaximum))<<32
		outer.set(ctrlLoopSavedRange, true)
		f.ctrl[len(f.ctrl)-1].set(ctrlLoopOuterRange, true)
	}
	f.loopAddressLocal, f.loopAddressMaximum, f.loopAddressDepth = local, maximum, len(f.ctrl)
	if f.ctrl[len(f.ctrl)-1].has(ctrlLoopOuterRange) {
		f.seedEnclosingLoopAddressMaxima()
	}
	return true
}

func (f *fn) restoreEnclosingLoopAddressRange(last int) {
	for i := last - 1; i >= 0; i-- {
		if f.ctrl[i].has(ctrlLoopSavedRange) {
			word := f.ctrlMerge(&f.ctrl[i]).loopPinPlan
			f.loopAddressLocal, f.loopAddressMaximum, f.loopAddressDepth = uint32(word), word>>32, i+1
			return
		}
	}
}
