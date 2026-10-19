//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"os"
)

var bmi2ShiftEnabled = os.Getenv("WAGO_AMD64_BMI2_SHIFT") == "1"

func (f *fn) tryBMI2Shift(node *elem, dest Reg) (Reg, bool) {
	if !f.opt(optBMI2Shift) || f.sc == nil || !f.sc.amd64Features.Has(shared.AMD64BMI2) {
		return regNone, false
	}
	var pp byte
	switch node.deferredOp() {
	case opShl:
		pp = 1
	case opShrS:
		pp = 2
	case opShrU:
		pp = 3
	default:
		return regNone, false
	}
	// Preserve WebAssembly evaluation order, keeping the value spillable while
	// the count expression uses any fixed-role arithmetic registers.
	left, right := node.arg0, node.arg1
	f.materialize(left)
	count, owned := f.materializeRead(right)
	saved := f.pinned
	f.pinned = f.pinned.add(count)
	value := f.materialize(left)
	result := dest
	if result == regNone {
		result = value
	}
	f.a.ShiftX(result, value, count, pp, node.valueType().is64())
	f.pinned = saved
	if owned && count != result {
		f.release(count)
	}
	if value != result {
		f.release(value)
	}
	f.consumeBlockBelow(node)
	f.occupy(node, result)
	node.setDeferredOp(opNone)
	f.stats.peep("bmi2-shift")
	return result, true
}
