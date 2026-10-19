//go:build amd64

package amd64

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"os"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// Test-only assertion hook: a declined guard traps instead of silently taking
// the scalar fallback, proving that qualifying fixtures exercise the prefix.
var invariantStoreAssertFast bool

var loopInvariantStoreEnabled = os.Getenv("WAGO_AMD64_LOOP_INVARIANT_STORE") == "1"
var loopInvariantStoreUnrollEnabled = os.Getenv("WAGO_AMD64_LOOP_INVARIANT_STORE_UNROLL") == "1"

const invariantStoreUnrollFactor = 4

func invariantLocal(b []byte, op byte, local int) []byte {
	return binary.AppendUvarint(append(b, op), uint64(local))
}

func invariantConst(b []byte, op byte, v int64) []byte {
	b = append(b, op)
	for {
		x := byte(v & 127)
		v >>= 7
		if v == 0 && x&64 == 0 || v == -1 && x&64 != 0 {
			return append(b, x)
		}
		b = append(b, x|128)
	}
}

func invariantAffine(b []byte, a integerMemoryAddress) []byte {
	b = invariantConst(b, 0x41, int64(int32(a.constant)))
	for local, c := range a.coeff {
		if c == 0 {
			continue
		}
		b = invariantLocal(b, 0x20, local)
		if c != 1 {
			b = invariantConst(b, 0x41, int64(int32(c)))
			b = append(b, 0x6c)
		}
		b = append(b, 0x6a)
	}
	return b
}

// Build an opt-in experimental version without changing the caller's original
// loop. Every failed guard jumps past this prefix; one original iteration after
// success publishes the final store and restores all source-visible locals.
func invariantStorePrefix(body []byte, p invariantStorePlan, locals int, classifier wasm.ModuleInstructionClassifier) []byte {
	b, _ := invariantStorePrefixWithLoads(body, p, locals, classifier)
	return b
}

func invariantStorePrefixWithLoads(body []byte, p invariantStorePlan, locals int, classifier wasm.ModuleInstructionClassifier) ([]byte, []uint32) {
	return invariantStorePrefixPolicy(body, p, locals, classifier, false)
}

func invariantStorePrefixPolicy(body []byte, p invariantStorePlan, locals int, classifier wasm.ModuleInstructionClassifier, unroll bool) ([]byte, []uint32) {
	var loads []uint32
	count, dst, dstEnd, src, srcEnd := locals, locals+1, locals+2, locals+3, locals+4
	b := []byte{0x02, 0x40}
	fail := func() {
		if invariantStoreAssertFast {
			b = append(b, 0x04, 0x40, 0x00, 0x0b)
		} else {
			b = append(b, 0x0d, 0)
		}
	}
	get := func(x int) { b = invariantLocal(b, 0x20, x) }
	set := func(x int) { b = invariantLocal(b, 0x21, x) }
	tee := func(x int) { b = invariantLocal(b, 0x22, x) }
	i32 := func(v uint32) { b = invariantConst(b, 0x41, int64(int32(v))) }
	i64 := func(v uint64) { b = invariantConst(b, 0x42, int64(v)) }
	memSize := func() { b = append(b, 0x3f, 0, 0xad); i64(16); b = append(b, 0x86) }
	limit, _ := p.p.affine(p.p.limit)
	b = invariantAffine(b, limit)
	tee(count)
	get(p.p.induction)
	b = append(b, 0x4d)
	fail()
	get(count)
	get(p.p.induction)
	b = append(b, 0x6b)
	set(count)
	if p.p.step != 1 {
		get(count)
		i32(p.p.step - 1)
		b = append(b, 0x71)
		fail()
		get(count)
		i32(uint32(bits.TrailingZeros32(p.p.step)))
		b = append(b, 0x76)
		set(count)
	}
	get(count)
	i32(8)
	b = append(b, 0x49)
	fail()
	target := p.p.stores[0]
	b = invariantAffine(b, target.address)
	b = append(b, 0xad)
	if target.offset != 0 {
		i64(target.offset)
		b = append(b, 0x7c)
	}
	tee(dst)
	i64(4)
	b = append(b, 0x7c)
	tee(dstEnd)
	memSize()
	b = append(b, 0x56)
	fail()
	// Guard all original iterations, not only the store-free prefix: otherwise
	// a trapping final scalar iteration could expose missing partial writes.
	for _, l := range p.p.loads[:p.p.loadN] {
		b = invariantAffine(b, l.address)
		b = append(b, 0xad)
		set(src)
		get(count)
		b = append(b, 0xad)
		i64(1)
		b = append(b, 0x7d)
		i64(uint64(l.stride))
		b = append(b, 0x7e)
		get(src)
		b = append(b, 0x7c)
		i64(4)
		b = append(b, 0x7c)
		tee(srcEnd)
		i64(1 << 32)
		b = append(b, 0x56)
		fail()
		if l.offset != 0 {
			get(src)
			i64(l.offset)
			b = append(b, 0x7c)
			set(src)
			get(srcEnd)
			i64(l.offset)
			b = append(b, 0x7c)
			set(srcEnd)
		}
		get(srcEnd)
		memSize()
		b = append(b, 0x56)
		fail()
		get(srcEnd)
		get(dst)
		b = append(b, 0x58)
		get(dstEnd)
		get(src)
		b = append(b, 0x58, 0x72, 0x45)
		fail()
	}
	emitBody := func() bool {
		r := wasm.NewReader(body[:p.backedge])
		for r.HasNext() {
			at := r.Offset()
			op, _ := r.Byte()
			var imm wasm.InstructionImmediate
			if classifier.ClassifyInto(r, op, &imm) != nil {
				return false
			}
			if op == 0x36 {
				b = append(b, 0x1a, 0x1a)
			} else {
				if op == 0x28 {
					loads = append(loads, uint32(len(b)))
				}
				b = append(b, body[at:r.Offset()]...)
			}
		}
		b = append(b, 0x1a) // discard the original latch predicate
		return true
	}
	if unroll {
		// The prefix executes trips-1 iterations. Peel its remainder before
		// full groups. At least one full group remains because trips>=8.
		// The existing all-iteration guards authorize each duplicated load.
		get(count)
		i32(1)
		b = append(b, 0x6b)
		i32(invariantStoreUnrollFactor - 1)
		b = append(b, 0x71)
		set(count)
		for peel := uint32(0); peel < invariantStoreUnrollFactor-1; peel++ {
			get(count)
			i32(peel)
			b = append(b, 0x4b, 0x04, 0x40)
			if !emitBody() {
				return nil, nil
			}
			b = append(b, 0x0b)
		}
	}
	// Reuse the source induction variable for the prefix latch. A private
	// decrementing trip counter would add a loop-carried update and store.
	b = invariantAffine(b, limit)
	i32(p.p.step)
	b = append(b, 0x6b)
	set(count) // now the invariant stop value, one source step before the limit
	b = append(b, 0x03, 0x40)
	if !emitBody() {
		return nil, nil
	}
	if unroll {
		for group := 1; group < invariantStoreUnrollFactor; group++ {
			if !emitBody() {
				return nil, nil
			}
		}
	}
	get(p.p.induction)
	get(count)
	b = append(b, 0x47)
	b = append(b, 0x0d, 0, 0x0b, 0x0b)
	return b, loads
}

// Experimental source-versioning entry point. Normal compilation must reject
// interruptible/profile/GC/custom modes before calling it. Original-PC mapping
// for the generated prefix is not yet qualified, so profiling stays unchanged.
// The immutable source module and original checked loops remain untouched.
func prototypeInvariantStoreModule(m *wasm.Module) (*wasm.Module, int) {
	module, n, _ := invariantStoreModuleWithLoads(m)
	return module, n
}

func invariantStoreModuleWithLoads(m *wasm.Module) (*wasm.Module, int, [][]uint32) {
	return invariantStoreModuleWithLoadPolicy(m, false)
}

func invariantStoreModuleWithLoadPolicy(m *wasm.Module, unroll bool) (*wasm.Module, int, [][]uint32) {
	if len(m.Memories) != 1 || m.ImportedMemCount() != 0 || m.Memories[0].Shared || m.Memories[0].Limits.Addr64 || len(m.BranchHints) != 0 {
		return m, 0, nil
	}
	classifier := wasm.NewModuleInstructionClassifier(m, true)
	var clone *wasm.Module
	var guardedLoads [][]uint32
	changed := 0
	for i, body := range m.Code {
		if len(body.BodyBytes) > 8192 || !bytes.Contains(body.BodyBytes, []byte{0x03, 0x40}) || bytes.IndexByte(body.BodyBytes, 0x36) < 0 {
			continue
		}
		ft, ok := m.LocalFuncType(i)
		if !ok || len(ft.Params) > 32 {
			continue
		}
		numeric, count := true, uint64(len(ft.Params))
		for _, typ := range ft.Params {
			if !wasm.EqualValType(typ, wasm.I32) {
				numeric = false
			}
		}
		for _, run := range body.Locals.Runs {
			count += uint64(run.Count)
			if !wasm.EqualValType(run.Type, wasm.I32) {
				numeric = false
			}
		}
		if !numeric || count > 32 {
			continue
		}
		types := make([]machineType, 0, int(count))
		for _, typ := range ft.Params {
			types = append(types, mtOf(typ))
		}
		for _, run := range body.Locals.Runs {
			for k := uint32(0); k < run.Count; k++ {
				types = append(types, mtOf(run.Type))
			}
		}
		if len(types) > 32 {
			continue
		}
		f := fn{nLocals: len(types), localType: types, classifier: classifier}
		r := wasm.NewReader(body.BodyBytes)
		var result []byte
		var loadSites []uint32
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
			if op != 0x03 || block != 0x40 || patches == 8 {
				continue
			}
			plan, ok := f.inspectInvariantStoreLoop(r)
			if !ok {
				continue
			}
			prefix, proof := invariantStorePrefixPolicy(body.BodyBytes[r.Offset():], plan, len(types), classifier, unroll)
			if len(prefix) == 0 || len(result)+len(prefix) > 16384 {
				continue
			}
			result = append(result, body.BodyBytes[last:at]...)
			for _, pc := range proof {
				loadSites = append(loadSites, uint32(len(result))+pc)
			}
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
			guardedLoads = make([][]uint32, len(m.Code))
		}
		copyBody := body
		copyBody.BodyBytes = result
		copyBody.Locals.Runs = append(append([]wasm.LocalRun{}, body.Locals.Runs...), wasm.LocalRun{Count: 1, Type: wasm.I32}, wasm.LocalRun{Count: 4, Type: wasm.I64})
		decl := binary.AppendUvarint(nil, uint64(len(copyBody.Locals.Runs)))
		for _, run := range copyBody.Locals.Runs {
			decl = binary.AppendUvarint(decl, uint64(run.Count))
			decl = append(decl, 0x7f)
		}
		copyBody.LocalDeclBytes = uint32(len(decl))
		for j := range loadSites {
			loadSites[j] += copyBody.LocalDeclBytes
		}
		guardedLoads[i] = loadSites
		clone.Code[i] = copyBody
		changed += patches
	}
	if clone == nil {
		return m, 0, nil
	}
	return clone, changed, guardedLoads
}

func (f *fn) guardedInvariantLoad() bool {
	if f.sc == nil || f.localBase != 0 || f.traceFuncIdx != uint32(f.globalIdx) {
		return false
	}
	for _, pc := range f.sc.invariantLoadPCs {
		if pc == f.wasmPC {
			return true
		}
		if pc > f.wasmPC {
			break
		}
	}
	return false
}
