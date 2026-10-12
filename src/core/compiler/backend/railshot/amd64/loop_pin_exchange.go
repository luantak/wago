//go:build amd64

package amd64

import (
	"os"
	"runtime"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// A small straight-line loop may borrow an inactive function pin. Its previous
// value is homed before the loop and restored on the loop's only exit. Integer
// and floating-point banks stay separate, while widths may share a register.
// WAGO_AMD64_NO_LOOP_PIN_EXCHANGE=1 is the rollback switch.
var loopPinExchangeEnabled = runtime.GOOS == "linux" && os.Getenv("WAGO_AMD64_NO_LOOP_PIN_EXCHANGE") != "1"

const loopPinBits = 17

func loopPinEntry(old, new int, reg Reg) uint64 {
	return uint64(old+1) | uint64(new)<<7 | uint64(reg)<<13
}

func (f *fn) planLoopPinExchange(r *wasm.Reader) (uint64, uint64) {
	if f.nLocals == 0 || f.nLocals > 64 || f.localBase != 0 || f.moduleEH || len(f.customInstructions) != 0 || f.intervalActive != 0 || len(f.pinnedLocals) == 0 {
		return 0, 0
	}
	// If every initialized scalar local already has a register, or no pin
	// shares an unpinned local's bank, the body cannot benefit from a lease.
	var candidates, pins uint8
	for x, local := range f.locals[:f.nLocals] {
		if !loopPinScalar(f.localType[x]) || local.state == lsConstZero {
			continue
		}
		bank := uint8(1)
		if f.localType[x].isFloat() {
			bank = 2
		}
		if local.reg == regNone {
			candidates |= bank
		} else {
			pins |= bank
		}
	}
	if candidates&pins == 0 {
		return 0, 0
	}
	reads, writes, written, ok := shared.LoopLocalUsage(r, f.classifier, f.nLocals)
	if !ok {
		return 0, 0
	}
	var plan uint64
	var selected uint64
	for count := 0; count < 2; count++ {
		oldByBank := [2]int{-1, -1}
		for _, old := range f.pinnedLocals {
			if selected&(uint64(1)<<old) != 0 || !loopPinScalar(f.localType[old]) || reads[old] != 0 || writes[old] != 0 || f.locals[old].state == lsConstZero {
				continue
			}
			bank := 0
			if f.localType[old].isFloat() {
				bank = 1
			}
			if oldByBank[bank] == -1 {
				oldByBank[bank] = old
			}
		}
		bestNew, bestOld, bestScore := -1, -1, 0
		for x := 0; x < f.nLocals; x++ {
			if selected&(uint64(1)<<x) != 0 || f.locals[x].reg != regNone || !loopPinScalar(f.localType[x]) || f.locals[x].state == lsConstZero {
				continue
			}
			score := int(reads[x])*2 + int(writes[x])
			if score <= bestScore || score < 6 {
				continue
			}
			bank := 0
			if f.localType[x].isFloat() {
				bank = 1
			}
			if old := oldByBank[bank]; old != -1 {
				bestNew, bestOld, bestScore = x, old, score
			}
		}
		if bestNew == -1 {
			break
		}
		plan |= loopPinEntry(bestOld, bestNew, f.locals[bestOld].reg) << (count * loopPinBits)
		selected |= uint64(1)<<bestOld | uint64(1)<<bestNew
	}
	return plan, written
}

func (f *fn) applyLoopPinExchange(plan uint64) {
	for plan != 0 {
		entry := plan & ((1 << loopPinBits) - 1)
		old := int(entry&0x7f) - 1
		newLocal := int((entry >> 7) & 0x3f)
		reg := Reg((entry >> 13) & 0xf)
		if !f.usesCalls || f.locals[old].state == lsReg {
			f.storeLocalReg(old, reg, f.locals[old].isFloat)
		}
		f.locals[old].reg = regNone
		f.locals[old].state = lsMem
		f.locals[newLocal].reg = reg
		f.locals[newLocal].isFloat = f.localType[newLocal].isFloat()
		f.loadLocalReg(newLocal, reg, f.locals[newLocal].isFloat)
		f.locals[newLocal].state = lsStackReg
		for i, x := range f.pinnedLocals {
			if x == old {
				f.pinnedLocals[i] = newLocal
				break
			}
		}
		plan >>= loopPinBits
	}
}

func (f *fn) restoreLoopPinExchange(plan uint64, reachable bool) {
	for plan != 0 {
		entry := plan & ((1 << loopPinBits) - 1)
		old := int(entry&0x7f) - 1
		newLocal := int((entry >> 7) & 0x3f)
		reg := Reg((entry >> 13) & 0xf)
		if reachable {
			if !f.usesCalls || f.locals[newLocal].state == lsReg {
				f.storeLocalReg(newLocal, reg, f.locals[newLocal].isFloat)
			}
			f.loadLocalReg(old, reg, f.locals[old].isFloat)
		}
		f.locals[newLocal].reg = regNone
		f.locals[newLocal].state = lsMem
		f.locals[old].reg = reg
		f.locals[old].state = lsStackReg
		for i, x := range f.pinnedLocals {
			if x == newLocal {
				f.pinnedLocals[i] = old
				break
			}
		}
		plan >>= loopPinBits
	}
}

func loopPinScalar(typ machineType) bool {
	return typ == mtI32 || typ == mtI64 || typ == mtF32 || typ == mtF64
}
