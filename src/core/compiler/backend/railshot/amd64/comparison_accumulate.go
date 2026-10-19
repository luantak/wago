//go:build amd64

package amd64

import "os"

var comparisonAccumulateEnabled = os.Getenv("WAGO_AMD64_COMPARISON_ACCUMULATE") == "1"

// Normalize unsigned relations to carry or inverted carry without moving tree
// nodes. Evaluate both comparison operands in Wasm order before swapping only
// the native CMP inputs. The bounded pure operands cannot trap or claim fixed
// division/variable-shift registers under the pinned accumulator.
func (f *fn) tryComparisonAccumulate(node *elem, dest Reg) Reg {
	if !f.opt(optComparisonAccumulate) || node.valueType() != mtI32 || (node.deferredOp() != opAdd && node.deferredOp() != opSub) || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return regNone
	}
	base, cmp := node.arg0, node.arg1
	if !booleanReady(base) || base.st.typ != mtI32 || !cmp.isDeferred() {
		return regNone
	}
	op := cmp.deferredOp()
	if op != opLtU && op != opGeU && op != opGtU && op != opLeU {
		return regNone
	}
	for _, e := range []*elem{cmp.arg0, cmp.arg1} {
		if _, ok := pureSelectCost(e, 6); !ok {
			return regNone
		}
	}
	out := f.materialize(base)
	saved := f.pinned
	f.pinned = f.pinned.add(out)
	a, ao := f.materializeRead(cmp.arg0)
	f.pinned = f.pinned.add(a)
	b, bo := f.materializeRead(cmp.arg1)
	x, y := a, b
	if op == opGtU || op == opLeU {
		x, y = y, x
	}
	if cmp.arg0.st.typ == mtI64 {
		f.a.Cmp64(x, y)
	} else {
		f.a.Cmp32(x, y)
	}
	digit, imm := byte(2), int32(0)
	if node.deferredOp() == opSub {
		digit = 3
	}
	if op == opGeU || op == opLeU {
		digit ^= 1
		imm = -1
	}
	f.a.AluRI(digit, out, imm, false)
	f.pinned = saved
	if ao {
		f.release(a)
	}
	if bo {
		f.release(b)
	}
	f.consumeBlockBelow(cmp)
	f.erase(cmp)
	f.erase(base)
	if dest != regNone && dest != out {
		f.moveInt(dest, out, mtI32)
		f.release(out)
		out = dest
	}
	f.occupy(node, out)
	node.setDeferredOp(opNone)
	node.st.setValueFacts(factUpper32Zero)
	f.stats.peep("comparison-accumulate")
	return out
}
