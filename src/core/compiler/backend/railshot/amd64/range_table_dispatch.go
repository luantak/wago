//go:build amd64

package amd64

type branchTableRange struct {
	last  int
	label uint32
}

// Restrict direct dispatch to large, low-entropy tables: at most three target
// runs, each amortized over at least three table entries on average. Dense or
// alternating targets retain constant-time indirect dispatch. The plan uses
// only label identity, never guest instructions or workload identity.
func branchTableRanges(labels []uint32) (ranges [3]branchTableRange, n int) {
	if len(labels) < 8 {
		return ranges, 0
	}
	for i, label := range labels {
		if n == 0 || ranges[n-1].label != label {
			if n == len(ranges) {
				return ranges, 0
			}
			ranges[n] = branchTableRange{label: label}
			n++
		}
		ranges[n-1].last = i
	}
	if len(labels) < n*3 {
		return ranges, 0
	}
	return ranges, n
}

func (f *fn) emitRangeTableDispatch(labels []uint32, def uint32, index Reg, emitCase func(uint32)) bool {
	if f.policy.CompactNative {
		return false
	}
	ranges, n := branchTableRanges(labels)
	if n == 0 {
		return false
	}
	// Exact singleton exceptions implicitly reject out-of-table indices. A
	// separate bounds check would only duplicate the default decision.
	singletons, first := true, 0
	for _, run := range ranges[:n] {
		if run.label != def && run.last != first {
			singletons = false
		}
		first = run.last + 1
	}
	var sites [3]int
	var labelsAtSite [3]uint32
	nSite := 0
	fallthroughLabel := def
	if singletons {
		for _, run := range ranges[:n] {
			if run.label == def {
				continue
			}
			f.a.AluRI(cmpDigit, index, int32(run.last), false)
			sites[nSite] = f.a.JccPlaceholder(condE)
			labelsAtSite[nSite] = run.label
			nSite++
		}
		f.stats.peep("range-table-singletons")
	} else {
		// Bound the index before partitioning it by inclusive run endpoints.
		f.a.AluRI(cmpDigit, index, int32(len(labels)), false)
		sites[nSite], labelsAtSite[nSite] = f.a.JccPlaceholder(condAE), def
		nSite++
		for i := 0; i < n-1; i++ {
			f.a.AluRI(cmpDigit, index, int32(ranges[i].last), false)
			sites[nSite], labelsAtSite[nSite] = f.a.JccPlaceholder(condBE), ranges[i].label
			nSite++
		}
		fallthroughLabel = ranges[n-1].label
	}
	// All tests precede transfers, and each distinct label gets exactly one
	// transfer stub even when multiple runs and the default share that target.
	var emittedLabels [4]uint32
	var emittedAt [4]int
	nEmitted := 0
	emitShared := func(label uint32) int {
		for i := 0; i < nEmitted; i++ {
			if emittedLabels[i] == label {
				return emittedAt[i]
			}
		}
		at := f.a.Len()
		emittedLabels[nEmitted], emittedAt[nEmitted] = label, at
		nEmitted++
		emitCase(label)
		return at
	}
	emitShared(fallthroughLabel)
	for i := 0; i < nSite; i++ {
		f.a.PatchRel32(sites[i], emitShared(labelsAtSite[i]))
	}
	f.stats.peep("range-table-dispatch")
	return true
}
