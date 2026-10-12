//go:build arm64

package arm64

import "github.com/wago-org/wago/src/core/compiler/wasm"

const (
	// Guards, eight-vector body, vector cleanup, and local commits.
	map32CodeReserve          = 768
	map32FunctionReserveLimit = 4 * map32CodeReserve
	map32ModuleReserveLimit   = 1 << 20
	map32I32HintShift         = 9
	map32F32HintShift         = 12
)

func (h funcHints) map32HintCount() int {
	return int(h.immediateFreeOps>>map32I32HintShift&7) + int(h.immediateFreeOps>>map32F32HintShift&7)
}

// The hint scan records a conservative shape. Emission still runs the full
// planner and all runtime guards.
func directMap32CodeReserve(m *wasm.Module, index int, h funcHints, opts *CompileOptions) int {
	if h.map32HintCount() == 0 || len(m.Code[index].BodyBytes) > regionLoopMaxBytes || h.localCount > 256 || h.flags.has(hintModuleEH) || opts.Interruptible || len(opts.CustomInstructions) != 0 || opts.GCFrameRoots.Function(index) != nil {
		return 0
	}
	mem, ok := m.MemoryType(0)
	if !ok || mem.Shared || mem.Limits.Addr64 {
		return 0
	}
	return h.map32HintCount() * map32CodeReserve
}

const (
	map32HintActive uint16 = 1 << iota
	map32HintInvalid
	map32HintLoadI32
	map32HintLoadF32
	map32HintStoreI32
	map32HintStoreF32
	map32HintMulI32
	map32HintMulF32
	map32HintBranch
)

func (s *byteBodyScanner) noteMap32Memory(op byte, imm wasm.InstructionImmediate) {
	if s.map32Shape == 0 {
		return
	}
	var bit uint16
	switch op {
	case 0x28:
		bit = map32HintLoadI32
	case 0x2a:
		bit = map32HintLoadF32
	case 0x36:
		bit = map32HintStoreI32
	case 0x38:
		bit = map32HintStoreF32
	default:
		s.map32Shape |= map32HintInvalid
		return
	}
	if s.map32Shape&bit != 0 || imm.MemIndex != 0 || imm.MemOffset > uint64(^uint32(0)) {
		s.map32Shape |= map32HintInvalid
	}
	s.map32Shape |= bit
}

func (s *byteBodyScanner) finishMap32Hint() {
	shape := s.map32Shape
	if shape&(map32HintActive|map32HintInvalid|map32HintBranch) != map32HintActive|map32HintBranch || s.h.map32HintCount() == 4 {
		return
	}
	const memoryMask = map32HintLoadI32 | map32HintLoadF32 | map32HintStoreI32 | map32HintStoreF32
	shift := 0
	if shape&memoryMask == map32HintLoadI32|map32HintStoreI32 && shape&map32HintMulI32 != 0 {
		shift = map32I32HintShift
	}
	if shape&memoryMask == map32HintLoadF32|map32HintStoreF32 && shape&map32HintMulF32 != 0 {
		shift = map32F32HintShift
	}
	if shift != 0 {
		s.h.immediateFreeOps += 1 << shift
	}
}
