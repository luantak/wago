//go:build linux && amd64 && !tinygo

package runtime

import (
	"syscall"
	"unsafe"
)

const maxSparseJobMemoryBytes = 4 << 20

func init() { sparseJobMemoryReclaim = reclaimSparseJobMemory }

func jobMemoryResidency(b, vec []byte) error {
	if len(b) == 0 {
		return nil
	}
	if len(vec) < (len(b)+pageSize-1)/pageSize {
		return syscall.EINVAL
	}
	_, _, errno := syscall.Syscall(syscall.SYS_MINCORE, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&vec[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

func reclaimSparseJobMemory(b []byte) bool {
	if len(b) == 0 || len(b) > maxSparseJobMemoryBytes || len(b)%pageSize != 0 || uintptr(unsafe.Pointer(&b[0]))%pageSize != 0 {
		return false
	}
	var bitmap [maxSparseJobMemoryBytes / pageSize]byte
	vec := bitmap[:len(b)/pageSize]
	if jobMemoryResidency(b, vec) != nil {
		return false
	}
	resident, holes := 0, 0
	for i, v := range vec {
		if v&1 != 0 {
			resident++
		} else if i == 0 || vec[i-1]&1 != 0 {
			holes++
		}
	}
	// Retain no more physical backing than the existing small-range clear
	// policy, and bound syscall amplification for fragmented working sets.
	if resident > jobMemoryReclaimThreshold/pageSize || holes > 8 {
		return false
	}
	return zeroSparseJobMemorySnapshot(b, vec)
}

// Residency need not remain accurate. Every page is either explicitly cleared
// or discarded to anonymous zero-fill; swapped/nonresident data is never left
// intact. Any partial failure selects the caller's whole-range fallback.
func zeroSparseJobMemorySnapshot(b, vec []byte) bool {
	for i := 0; i < len(vec); {
		start := i
		resident := vec[i]&1 != 0
		for i < len(vec) && (vec[i]&1 != 0) == resident {
			i++
		}
		span := b[start*pageSize : i*pageSize]
		if resident {
			clear(span)
		} else if madviseDontNeed(span) != nil {
			return false
		}
	}
	return true
}
