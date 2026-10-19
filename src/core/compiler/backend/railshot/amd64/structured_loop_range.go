//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var structuredLoopRangeEnabled = os.Getenv("WAGO_AMD64_STRUCTURED_LOOP_RANGE") == "1"
var nestedLoopRangeEnabled = os.Getenv("WAGO_AMD64_NESTED_LOOP_RANGE") == "1"

// Prove one uniform range for a monotone i32 induction local, including its
// final increment. Balanced if arms may compute values or perform memory work,
// but all control transfers except the final loop latch are rejected.
func (f *fn) planStructuredLoopRange(r *wasm.Reader) (uint32, uint64) {
	if !f.opt(optStructuredLoopRange) || f.nLocals > 64 || f.localBase != 0 || f.moduleEH ||
		f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return 0, 0
	}
	nested := f.opt(optNestedLoopRange)
	if f.loopAddressDepth != 0 && !nested {
		return 0, 0
	}
	// The final proof needs an entry bound. Avoid decoding an entire loop
	// when neither straight-line facts nor a lazy-zero local can supply one.
	if f.localAddressMaximaValid == 0 {
		known := false
		for x := 0; x < f.nLocals; x++ {
			if f.localType[x] == mtI32 && f.locals[x].state == lsConstZero {
				known = true
				break
			}
		}
		if !known {
			return 0, 0
		}
	}
	type instruction struct {
		op    byte
		depth uint8
		value uint32
	}
	var tail [8]instruction
	var writes [64]uint16
	scan := *r
	depth := 0
	for n := 0; n < 256 && scan.Offset()-r.Offset() < 2048; n++ {
		op, err := scan.Byte()
		if err != nil {
			return 0, 0
		}
		if op == 0x0b && depth == 0 {
			if n < 7 {
				return 0, 0
			}
			at := func(back int) instruction { return tail[(n-back)%len(tail)] }
			branch, cmp, limit := at(1), at(2), at(3)
			if branch.op != 0x0d || branch.value != 0 || (cmp.op != 0x47 && cmp.op != 0x48 && cmp.op != 0x49) || limit.op != 0x41 || limit.value == 0 || limit.value > 0x7fffffff {
				return 0, 0
			}
			set, add, step, get := at(4), at(5), at(6), at(7)
			if set.op == 0x20 && n >= 8 {
				read := set
				set, add, step, get = at(5), at(6), at(7), at(8)
				if set.op != 0x21 || set.value != read.value {
					return 0, 0
				}
			} else if set.op != 0x22 {
				return 0, 0
			}
			x := set.value
			if x >= uint32(f.nLocals) || f.localType[x] != mtI32 || writes[x] != 1 || get.op != 0x20 || get.value != x || step.op != 0x41 || step.value == 0 || step.value > 0x7fffffff || add.op != 0x6a {
				return 0, 0
			}
			maximum := uint64(^uint32(0))
			if f.locals[x].state == lsConstZero {
				maximum = 0
			}
			for i, fact := range f.localAddressMaxima {
				if f.localAddressMaximaValid&(1<<i) != 0 && fact.index == x {
					maximum = fact.maximum
				}
			}
			if maximum >= uint64(limit.value) {
				return 0, 0
			}
			bound := uint64(limit.value) - 1 + uint64(step.value)
			if bound > 0x7fffffff {
				return 0, 0
			}
			if cmp.op == 0x47 {
				if step.value != 1 && (maximum != 0 || limit.value%step.value != 0) {
					return 0, 0
				}
				bound = uint64(limit.value)
			}
			return x + 1, bound
		}
		if n > 0 && tail[(n-1)%len(tail)].op == 0x0d && tail[(n-1)%len(tail)].depth == 0 {
			return 0, 0
		}
		switch op {
		case 0x02, 0x03, 0x04:
			if op != 0x04 && !nested {
				return 0, 0
			}
			depth++
			if depth > 8 {
				return 0, 0
			}
		case 0x05:
			if depth == 0 {
				return 0, 0
			}
		case 0x0b:
			depth--
		case 0x0c, 0x0d:
			if op == 0x0c && depth == 0 || depth != 0 && !nested {
				return 0, 0
			}
		default:
			if !loopPinAllowed(op) {
				return 0, 0
			}
		}
		look := scan
		var imm wasm.InstructionImmediate
		if f.classifier.ClassifyInto(&scan, op, &imm) != nil {
			return 0, 0
		}
		// Inner branches must remain strictly inside this loop. In particular,
		// an early continue to this loop could bypass the unique tail increment.
		if (op == 0x0c || op == 0x0d) && depth > 0 && imm.Index >= uint32(depth) {
			return 0, 0
		}
		value := imm.Index
		if op == 0x41 {
			v, err := look.I32()
			if err != nil {
				return 0, 0
			}
			value = uint32(v)
		}
		if op == 0x21 || op == 0x22 {
			if value >= uint32(f.nLocals) {
				return 0, 0
			}
			writes[value]++
		}
		tail[n%len(tail)] = instruction{op: op, depth: uint8(depth), value: value}
	}
	return 0, 0
}

func (f *fn) seedDeclaredLoopAddressZeros() {
	if !f.opt(optStructuredLoopRange) || f.nLocals > 64 || f.moduleEH || f.gcFrameRoots != nil || len(f.customInstructions) != 0 {
		return
	}
	// Wasm declared locals are logically zero, even when physical entry stores
	// are elided because their first use is an assignment. Existing writes and
	// control boundaries invalidate these ordinary straight-line facts.
	for x := f.nParams; x < f.nLocals; x++ {
		if f.localType[x] == mtI32 {
			f.rememberAddressMaximum(x, 0)
		}
	}
}
