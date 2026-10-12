package shared

import "github.com/wago-org/wago/src/core/compiler/wasm"

// LoopLocalUsage proves a bounded, scalar loop with one conditional backedge
// and only its fall-through exit. It neither consumes the caller's reader nor
// admits calls, nested control, bulk helpers or proposal instructions.
// This permits temporary register homes without changing any external edge.
func LoopLocalUsage(r *wasm.Reader, classifier wasm.ModuleInstructionClassifier, locals int) (reads, writes [64]uint8, written uint64, ok bool) {
	if locals <= 0 || locals > len(reads) {
		return
	}
	look := *r
	start := look.Offset()
	backedge := false
	var imm wasm.InstructionImmediate
	for step := 0; step < 96 && look.Offset()-start <= 512; step++ {
		op, err := look.Byte()
		if err != nil {
			return
		}
		if backedge {
			ok = op == 0x0b
			return
		}
		if op >= 0x20 && op <= 0x22 {
			index, err := look.U32()
			if err != nil || index >= uint32(locals) {
				return
			}
			if op == 0x20 {
				if reads[index] != 255 {
					reads[index]++
				}
			} else {
				if writes[index] != 255 {
					writes[index]++
				}
				written |= uint64(1) << index
			}
			continue
		}
		if op == 0x0d {
			label, err := look.U32()
			if err != nil || label != 0 {
				return
			}
			backedge = true
			continue
		}
		switch {
		case op == 0x01 || op == 0x1a || op == 0x1b, op >= 0x45 && op <= 0xc4:
			// Scalar operators have no immediates. Their original evaluation and
			// trap order remain unchanged; only local homes differ.
		case op >= 0x28 && op <= 0x3e, op >= 0x41 && op <= 0x44:
			if classifier.ClassifyInto(&look, op, &imm) != nil {
				return
			}
		default:
			return
		}
	}
	return
}
