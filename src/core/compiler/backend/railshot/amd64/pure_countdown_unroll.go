//go:build amd64

package amd64

import (
	"bytes"
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var pureCountdownUnrollEnabled = os.Getenv("WAGO_AMD64_PURE_COUNTDOWN_UNROLL") == "1"

const pureCountdownFactor = 4

type pureCountdownPlan struct {
	counter    uint32
	payloadEnd int
}

// Only total, local integer work may precede the unique final decrement and
// br_if0. Nested control, memory, calls and trapping arithmetic all decline.
func planPureCountdown(source *wasm.Reader, classifier wasm.ModuleInstructionClassifier, locals int) (pureCountdownPlan, bool) {
	if locals < 1 || locals > 64 {
		return pureCountdownPlan{}, false
	}
	scan := *source
	start := scan.Offset()
	type instruction struct {
		op    byte
		idx   uint32
		value int32
	}
	var tail [5]instruction
	var writes [64]uint8
	for n := 0; n < 256 && scan.Offset()-start <= 256; n++ {
		at := scan.Offset()
		op, err := scan.Byte()
		if err != nil {
			return pureCountdownPlan{}, false
		}
		immediate := scan
		var imm wasm.InstructionImmediate
		if classifier.ClassifyInto(&scan, op, &imm) != nil {
			return pureCountdownPlan{}, false
		}
		item := instruction{op: op, idx: imm.Index}
		if op == 0x41 {
			item.value, err = immediate.I32()
			if err != nil {
				return pureCountdownPlan{}, false
			}
		}
		if op >= 0x20 && op <= 0x22 {
			if imm.Index >= uint32(locals) {
				return pureCountdownPlan{}, false
			}
			if op != 0x20 && writes[imm.Index] != 255 {
				writes[imm.Index]++
			}
		}
		tail[n%len(tail)] = item
		if op == 0x0d {
			end, e := scan.Byte()
			if e != nil || end != 0x0b || imm.Index != 0 || n < 4 {
				return pureCountdownPlan{}, false
			}
			a, b, c, d := tail[(n-4)%5], tail[(n-3)%5], tail[(n-2)%5], tail[(n-1)%5]
			if a.op != 0x20 || b.op != 0x41 || d.op != 0x22 || d.idx != a.idx || writes[a.idx] != 1 ||
				!(c.op == 0x6a && b.value == -1 || c.op == 0x6b && b.value == 1) {
				return pureCountdownPlan{}, false
			}
			return pureCountdownPlan{counter: a.idx, payloadEnd: at}, true
		}
		allowed := op == 0x01 || op == 0x1a || op == 0x1b || op >= 0x20 && op <= 0x22 || op == 0x41 ||
			op >= 0x45 && op <= 0x4f || op >= 0x67 && op <= 0x6c || op >= 0x71 && op <= 0x78 || op == 0xc0 || op == 0xc1
		if !allowed {
			return pureCountdownPlan{}, false
		}
	}
	return pureCountdownPlan{}, false
}

func pureCountdownPrefix(payload []byte, counter uint32) []byte {
	b := make([]byte, 0, pureCountdownFactor*(len(payload)+1)+24)
	// Leave 1..factor original scalar iterations. Initial zero takes the
	// unchanged original path, including its modular wraparound semantics.
	b = append(b, 0x02, 0x40)
	b = invariantLocal(b, 0x20, int(counter))
	b = invariantConst(b, 0x41, pureCountdownFactor)
	b = append(b, 0x4d, 0x0d, 0, 0x03, 0x40)
	for i := 0; i < pureCountdownFactor; i++ {
		b = append(b, payload...)
		b = append(b, 0x1a)
	}
	b = invariantLocal(b, 0x20, int(counter))
	b = invariantConst(b, 0x41, pureCountdownFactor)
	b = append(b, 0x4b, 0x0d, 0, 0x0b, 0x0b)
	return b
}

func pureCountdownModule(m *wasm.Module) (*wasm.Module, int) {
	return pureCountdownModuleHinted(m, nil)
}

//go:noinline
func preparePureCountdown(m *wasm.Module, hints []funcHints, opts *CompileOptions) (*wasm.Module, int) {
	if opts.Interruptible || opts.Profile || opts.CompactNative || opts.GCFrameRoots != nil || opts.GCStructHelpers || opts.GCArrayHelpers || len(opts.Codegen.Module.GCTypeLayouts) != 0 || len(opts.CustomInstructions) != 0 {
		return m, 0
	}
	return pureCountdownModuleHinted(m, hints)
}

func pureCountdownModuleHinted(m *wasm.Module, hints []funcHints) (*wasm.Module, int) {
	if len(m.BranchHints) != 0 {
		return m, 0
	}
	classifier := wasm.NewModuleInstructionClassifier(m, true)
	var clone *wasm.Module
	changed := 0
	for i, body := range m.Code {
		// Reuse the mandatory hint scan as the negative effect filter. Pure
		// functions cannot shift another function's invariant-load proof PCs.
		if hints != nil && (i >= len(hints) || !hints[i].flags.has(hintHasLoop) || hints[i].flags.has(hintHasCall|hintTouchesMemory|hintModuleEH|hintHasSIMD)) {
			continue
		}
		if len(body.BodyBytes) > 8192 || !pureCountdownCandidate(body.BodyBytes) || !bytes.Contains(body.BodyBytes, []byte{0x03, 0x40}) {
			continue
		}
		ft, ok := m.LocalFuncType(i)
		if !ok {
			continue
		}
		locals, numeric := uint64(len(ft.Params)), true
		for _, typ := range ft.Params {
			numeric = numeric && wasm.EqualValType(typ, wasm.I32)
		}
		for _, run := range body.Locals.Runs {
			locals += uint64(run.Count)
			numeric = numeric && wasm.EqualValType(run.Type, wasm.I32)
		}
		if !numeric || locals == 0 || locals > 64 {
			continue
		}
		r := wasm.NewReader(body.BodyBytes)
		var result []byte
		last, patches := 0, 0
		for r.HasNext() {
			at := r.Offset()
			op, err := r.Byte()
			if err != nil {
				break
			}
			look := *r
			block, _ := look.Byte()
			var imm wasm.InstructionImmediate
			if classifier.ClassifyInto(r, op, &imm) != nil {
				break
			}
			if op != 0x03 || block != 0x40 || patches == 4 {
				continue
			}
			plan, ok := planPureCountdown(r, classifier, int(locals))
			if !ok {
				continue
			}
			prefix := pureCountdownPrefix(body.BodyBytes[r.Offset():plan.payloadEnd], plan.counter)
			if len(result)+len(prefix) > 4096 {
				continue
			}
			result = append(result, body.BodyBytes[last:at]...)
			result = append(result, prefix...)
			last = at
			patches++
		}
		if patches == 0 {
			continue
		}
		result = append(result, body.BodyBytes[last:]...)
		if clone == nil {
			c := *m
			clone = &c
			clone.Code = append([]wasm.Func{}, m.Code...)
		}
		copyBody := body
		copyBody.BodyBytes = result
		clone.Code[i] = copyBody
		changed += patches
	}
	if clone == nil {
		return m, 0
	}
	return clone, changed
}

// A negative prefilter only: false positives still receive the full proof.
// Canonical unit decrements are sufficient for this optional optimization;
// noncanonical constant encodings keep the unchanged scalar implementation.
// Avoid another bytecode walk of the overwhelmingly unrelated functions.
func pureCountdownCandidate(body []byte) bool {
	for _, pattern := range [2][4]byte{{0x41, 0x7f, 0x6a, 0x22}, {0x41, 1, 0x6b, 0x22}} {
		for offset := 0; offset < len(body); {
			i := bytes.Index(body[offset:], pattern[:])
			if i < 0 {
				break
			}
			i += offset + len(pattern)
			// Canonical local and label encodings. A decrement used as data
			// or followed by another condition is not a candidate latch.
			if i+4 <= len(body) && body[i] < 64 && body[i+1] == 0x0d && body[i+2] == 0 && body[i+3] == 0x0b {
				return true
			}
			offset = i
		}
	}
	return false
}
