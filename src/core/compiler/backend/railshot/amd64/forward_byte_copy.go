//go:build amd64

package amd64

import (
	"os"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

var forwardByteCopyEnabled = os.Getenv("WAGO_AMD64_FORWARD_BYTE_COPY") == "1"
var forwardByteCopyAssertFast bool

// Affine expressions are modulo i32; memory immediates are kept separately.
// Only eight locals actually used by the loop need symbolic state, regardless
// of the containing function's local count.
type byteCopyAffine struct {
	constant uint32
	coeff    [8]uint32
}

func (a byteCopyAffine) combine(b byteCopyAffine, subtract bool) byteCopyAffine {
	if subtract {
		a.constant -= b.constant
		for i := range a.coeff {
			a.coeff[i] -= b.coeff[i]
		}
	} else {
		a.constant += b.constant
		for i := range a.coeff {
			a.coeff[i] += b.coeff[i]
		}
	}
	return a
}

type byteCopyValue struct {
	kind byte // 0 affine, 1 loaded byte, 2 unsigned latch comparison
	a, b byteCopyAffine
	load int
	op   byte
}

type byteCopyAccess struct {
	address byteCopyAffine
	offset  uint64
}

type forwardByteCopyPlan struct {
	local              [8]int
	value              [8]byteCopyAffine
	step               [8]uint32
	n, copies, counter int
	written            uint8
	src, dst           [16]byteCopyAccess
	exact              bool
}

func (p *forwardByteCopyPlan) localIndex(x int) int {
	for i := 0; i < p.n; i++ {
		if p.local[i] == x {
			return i
		}
	}
	if p.n == len(p.local) {
		return -1
	}
	i := p.n
	p.n++
	p.local[i], p.value[i].coeff[i] = x, 1
	return i
}

func (f *fn) inspectForwardByteCopy(r *wasm.Reader) (p forwardByteCopyPlan, ok bool) {
	scan := regionLoopReader(*r)
	var stack [16]byteCopyValue
	depth, loads := 0, 0
	for instruction := 0; instruction < 256 && scan.HasNext(); instruction++ {
		op, err := scan.Byte()
		if err != nil {
			return p, false
		}
		look := scan
		var imm wasm.InstructionImmediate
		if f.classifier.ClassifyInto(&scan, op, &imm) != nil {
			return p, false
		}
		var v byteCopyValue
		switch op {
		case 0x01: // nop
			continue
		case 0x20, 0x21, 0x22:
			x := int(imm.Index)
			if x >= len(f.localType) || f.localType[x] != mtI32 {
				return p, false
			}
			i := p.localIndex(x)
			if i < 0 {
				return p, false
			}
			if op == 0x20 {
				v.a = p.value[i]
			} else {
				if depth == 0 || stack[depth-1].kind != 0 {
					return p, false
				}
				p.value[i], p.written = stack[depth-1].a, p.written|1<<i
				if op == 0x21 {
					depth--
				}
				continue
			}
		case 0x41:
			c, err := look.I32()
			if err != nil {
				return p, false
			}
			v.a.constant = uint32(c)
		case 0x6a, 0x6b: // i32 add/sub
			if depth < 2 || stack[depth-1].kind != 0 || stack[depth-2].kind != 0 {
				return p, false
			}
			depth -= 2
			v.a = stack[depth].a.combine(stack[depth+1].a, op == 0x6b)
		case 0x2d: // i32.load8_u; every read must precede its own store
			if depth == 0 || stack[depth-1].kind != 0 || loads != p.copies || loads == len(p.src) || imm.MemIndex != 0 || imm.MemOffset > 0xffffffff {
				return p, false
			}
			depth--
			p.src[loads] = byteCopyAccess{stack[depth].a, imm.MemOffset}
			v.kind, v.load = 1, loads
			loads++
		case 0x3a: // i32.store8
			if depth < 2 || stack[depth-2].kind != 0 || stack[depth-1].kind != 1 || stack[depth-1].load != p.copies || imm.MemIndex != 0 || imm.MemOffset > 0xffffffff {
				return p, false
			}
			depth -= 2
			p.dst[p.copies] = byteCopyAccess{stack[depth].a, imm.MemOffset}
			p.copies++
			continue
		case 0x47, 0x4b, 0x4f: // ne / gt_u / ge_u
			if depth != 2 || stack[0].kind != 0 || stack[1].kind != 0 {
				return p, false
			}
			v.kind, v.a, v.b, v.op = 2, stack[0].a, stack[1].a, op
			depth = 0
		case 0x0d:
			if imm.Index != 0 || depth != 1 || stack[0].kind != 2 || p.copies == 0 || p.copies != loads {
				return p, false
			}
			end, err := scan.Byte()
			if err != nil || end != 0x0b {
				return p, false
			}
			return proveForwardByteCopy(p, stack[0])
		default:
			return p, false
		}
		if depth == len(stack) {
			return p, false
		}
		stack[depth] = v
		depth++
	}
	return p, false
}

func proveForwardByteCopy(p forwardByteCopyPlan, latch byteCopyValue) (forwardByteCopyPlan, bool) {
	k := uint32(p.copies)
	if latch.b.coeff != [8]uint32{} ||
		!(latch.op == 0x4b && latch.b.constant == k-1 || latch.op == 0x4f && latch.b.constant == k || latch.op == 0x47 && latch.b.constant == 0) {
		return p, false
	}
	p.exact = latch.op == 0x47
	p.counter = -1
	for i := 0; i < p.n; i++ {
		var unit [8]uint32
		unit[i] = 1
		if p.written&(1<<i) != 0 && p.value[i].coeff == unit {
			p.step[i] = p.value[i].constant
			if latch.a == p.value[i] && p.step[i] == -k {
				p.counter = i
			}
		}
	}
	if p.counter < 0 {
		return p, false
	}
	for i := 0; i < p.n; i++ {
		if p.written&(1<<i) == 0 {
			continue
		}
		// Non-induction temporaries may depend on invariants and independent
		// induction variables, never on another loop-carried temporary.
		for j, c := range p.value[i].coeff {
			if c != 0 && p.written&(1<<j) != 0 && p.step[j] == 0 {
				return p, false
			}
		}
	}
	for _, streams := range [][16]byteCopyAccess{p.src, p.dst} {
		// Only invariants and independent induction variables may carry an
		// address across iterations. A temporary read before its new definition
		// is not invariant merely because that definition is affine.
		for i := 0; i < p.copies; i++ {
			for j, c := range streams[i].address.coeff {
				if c != 0 && p.written&(1<<j) != 0 && p.step[j] == 0 {
					return p, false
				}
			}
		}
		var stride uint32
		for j, c := range streams[0].address.coeff {
			stride += c * p.step[j]
		}
		if stride != k {
			return p, false
		}
		for i := 1; i < p.copies; i++ {
			a, b := streams[0], streams[i]
			if a.address.coeff != b.address.coeff || b.address.constant+uint32(b.offset)-a.address.constant-uint32(a.offset) != uint32(i) {
				return p, false
			}
		}
	}
	return p, true
}
