//go:build amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
)

const (
	map32CodeReserve          = 1024
	map32FunctionReserveLimit = 4 * map32CodeReserve
	map32ModuleReserveLimit   = 1 << 20
)

// Reserve guard/body space for small map functions. This is a sizing hint, not
// an emission decision: later register/home checks can still reject a plan.
// Larger functions retain the existing growth path. Both scans and storage are
// bounded; ordinary functions skip the scan through their existing hints.
func directMap32CodeReserve(m *wasm.Module, index int, h funcHints, opts CompileOptions, classifier wasm.ModuleInstructionClassifier) int {
	body := m.Code[index].BodyBytes
	if !h.flags.has(hintHasLoop) || h.flags.has(hintModuleEH) || len(body) == 0 || len(body) > regionLoopMaxBytes || h.localCount > 256 || opts.Interruptible || len(opts.CustomInstructions) != 0 || opts.GCFrameRoots.Function(index) != nil {
		return 0
	}
	// Every admitted map has an explicit i32.mul or f32.mul. A byte check
	// rejects common ordinary loops cheaply; false positives still need proof.
	if bytes.IndexByte(body, 0x6c) < 0 && bytes.IndexByte(body, 0x94) < 0 {
		return 0
	}
	mem, ok := m.MemoryType(0)
	if !ok || mem.Shared || mem.Limits.Addr64 {
		return 0
	}
	ft, ok := m.LocalFuncType(index)
	if !ok {
		return 0
	}
	var types [256]machineType
	n := 0
	for _, typ := range ft.Params {
		if n == len(types) {
			return 0
		}
		types[n] = mtOf(typ)
		n++
	}
	for _, run := range m.Code[index].Locals.Runs {
		if uint64(run.Count) > uint64(len(types)-n) {
			return 0
		}
		for j := uint32(0); j < run.Count; j++ {
			types[n] = mtOf(run.Type)
			n++
		}
	}
	reserve := 0
	r := wasm.ReaderFrom(body)
	for r.BytesLeft() > 0 && reserve < map32FunctionReserveLimit {
		op, err := r.Byte()
		if err != nil {
			return reserve
		}
		look := r
		var imm wasm.InstructionImmediate
		if classifier.ClassifyInto(&r, op, &imm) != nil {
			return reserve
		}
		if op != 3 {
			continue
		}
		bt, err := look.Byte()
		if err != nil || bt != 0x40 {
			continue
		}
		var p regionLoopPlan
		if inspectRegionLoop(r, types[:n], classifier, &p) && p.map32 && p.qualifyMap32() && (p.nodes[p.loads[0].node].op != 0x28 || opts.AMD64Features.Has(shared.AMD64SSE41)) {
			reserve += map32CodeReserve
		}
	}
	return reserve
}
