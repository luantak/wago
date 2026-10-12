//go:build arm64

package arm64

import (
	"encoding/binary"

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

const (
	regionLoopMaxOps    = 64
	regionLoopMaxBytes  = 512
	regionLoopMaxLocals = 8
)

type regionLoopNode struct {
	bits                  uint64
	pos                   uint32
	coeff                 [regionLoopMaxLocals]uint32
	op, left, right, uses uint8
}

type regionLoopLocal struct {
	index          uint32
	typ            machineType
	step           uint32
	initial, value uint8
	written        bool
}

type regionLoopStore struct {
	offset         uint64
	pos            uint32
	address, value uint8
}

type regionLoopLoad struct {
	stride      uint32
	node, bytes uint8
}

type regionLoopPlan struct {
	nodes                        [regionLoopMaxOps + 1]regionLoopNode
	locals                       [regionLoopMaxLocals]regionLoopLocal
	stores                       [1]regionLoopStore
	loads                        [1]regionLoopLoad
	events                       [regionLoopMaxOps]uint8
	eventN                       uint8
	endPC                        int
	nodeN, localN, storeN, loadN uint8
	counter, limit, predicate    uint8
	inputMask                    uint8
	zeroTerminated               bool
	headerCount                  uint8
}

// add appends a canonical node; affine constructors keep bits and uses exact.
func (p *regionLoopPlan) add(n regionLoopNode) uint8 {
	if p.nodeN == regionLoopMaxOps {
		return 0
	}
	p.nodeN++
	p.nodes[p.nodeN] = n
	return p.nodeN
}

func (p *regionLoopPlan) local(index uint32, typ machineType) (uint8, bool) {
	for i := uint8(0); i < p.localN; i++ {
		if p.locals[i].index == index {
			return i, true
		}
	}
	if p.localN == regionLoopMaxLocals || typ != mtI32 && typ != mtF32 {
		return 0, false
	}
	i := p.localN
	n := regionLoopNode{}
	if typ == mtI32 {
		n.coeff[i], n.uses = 1, 1<<i
	} else {
		n.op, n.bits, n.uses = 0x20, uint64(i), 1<<i
	}
	id := p.add(n)
	if id == 0 {
		return 0, false
	}
	p.locals[i] = regionLoopLocal{index: index, typ: typ, initial: id, value: id}
	p.localN++
	return i, true
}

func (p *regionLoopPlan) integer(op byte, a, b uint8) uint8 {
	x, y := p.nodes[a], p.nodes[b]
	if x.op != 0 || y.op != 0 {
		return 0
	}
	switch op {
	case 0x6a, 0x6b:
		if op == 0x6b {
			x.bits = uint64(uint32(x.bits) - uint32(y.bits))
		} else {
			x.bits = uint64(uint32(x.bits) + uint32(y.bits))
		}
		// The common induction update changes only the constant term.
		if y.uses == 0 {
			return p.add(x)
		}
		x.uses = 0
		for i := range x.coeff {
			if op == 0x6b {
				x.coeff[i] -= y.coeff[i]
			} else {
				x.coeff[i] += y.coeff[i]
			}
			if x.coeff[i] != 0 {
				x.uses |= 1 << i
			}
		}
	case 0x6c, 0x74:
		if op == 0x74 {
			if y.uses != 0 {
				return 0
			}
			y.bits = uint64(uint32(1) << (uint32(y.bits) & 31))
		} else if y.uses != 0 {
			x, y = y, x
		}
		if y.uses != 0 {
			return 0
		}
		x.bits = uint64(uint32(x.bits) * uint32(y.bits))
		if x.uses != 0 {
			x.uses = 0
			for i := range x.coeff {
				x.coeff[i] *= uint32(y.bits)
				if x.coeff[i] != 0 {
					x.uses |= 1 << i
				}
			}
		}
	default:
		return 0
	}
	return p.add(x)
}

func regionLoopReader(r wasm.Reader) wasm.Reader {
	size := r.BytesLeft()
	if size > regionLoopMaxBytes {
		size = regionLoopMaxBytes
	}
	data, _ := r.Bytes(size)
	return wasm.ReaderFrom(data)
}

// inspectRegionLoop reads one bounded empty-type loop without changing the
// caller's reader. It accepts only the scalar i32/f32 map grammar.
func inspectRegionLoop(r wasm.Reader, types []machineType, classifier wasm.ModuleInstructionClassifier, p *regionLoopPlan) bool {
	*p = regionLoopPlan{}
	start := r.Offset()
	r = regionLoopReader(r)
	var stack [16]uint8
	depth := 0
	for step := 0; step < regionLoopMaxOps && r.Offset() < regionLoopMaxBytes; step++ {
		position := uint32(start + r.Offset())
		op, err := r.Byte()
		if err != nil {
			return false
		}
		var imm wasm.InstructionImmediate
		// Decode only the planner's grammar. Constants are read once below;
		// memory immediates retain the module's width and multi-memory rules.
		switch op {
		case 0x20, 0x21, 0x22, 0x0c, 0x0d:
			imm.Index, err = r.U32()
		case 0x28, 0x2a, 0x36, 0x38:
			err = classifier.ClassifyInto(&r, op, &imm)
		}
		if err != nil {
			return false
		}
		var id uint8
		switch op {
		case 0x20, 0x21, 0x22:
			if uint64(imm.Index) >= uint64(len(types)) {
				return false
			}
			i, ok := p.local(imm.Index, types[imm.Index])
			if !ok {
				return false
			}
			if op == 0x20 {
				id = p.locals[i].value
			} else {
				if depth == 0 || types[imm.Index] == mtI32 && p.nodes[stack[depth-1]].op != 0 && !p.map32Int(stack[depth-1]) || types[imm.Index] == mtF32 && !p.map32Float(stack[depth-1]) {
					return false
				}
				p.locals[i].value, p.locals[i].written = stack[depth-1], true
				if op == 0x21 {
					depth--
				}
				continue
			}
		case 0x41:
			x, err := r.I32()
			if err != nil {
				return false
			}
			id = p.add(regionLoopNode{bits: uint64(uint32(x))})
		case 0x43:
			x, err := r.Bytes(4)
			if err != nil {
				return false
			}
			id = p.add(regionLoopNode{op: op, bits: uint64(binary.LittleEndian.Uint32(x))})
		case 0x28, 0x2a:
			if depth == 0 || imm.MemIndex != 0 || imm.MemOffset > uint64(^uint32(0)) {
				return false
			}
			depth--
			a := stack[depth]
			if p.nodes[a].op != 0 {
				return false
			}
			id = p.add(regionLoopNode{op: op, left: a, bits: imm.MemOffset, uses: p.nodes[a].uses})
		case 0x36, 0x38:
			if depth < 2 || p.storeN != 0 || imm.MemIndex != 0 || imm.MemOffset > uint64(^uint32(0)) {
				return false
			}
			depth -= 2
			a, b := stack[depth], stack[depth+1]
			if p.nodes[a].op != 0 || op == 0x38 && !p.map32Float(b) || op == 0x36 && !p.map32Int(b) {
				return false
			}
			p.stores[0] = regionLoopStore{offset: imm.MemOffset, pos: position, address: a, value: b}
			p.events[p.eventN] = 0x80
			p.eventN++
			p.storeN = 1
			continue
		case 0x92, 0x94, 0x6a, 0x6b, 0x6c, 0x74, 0x47:
			if depth < 2 {
				return false
			}
			depth -= 2
			a, b := stack[depth], stack[depth+1]
			if op == 0x92 || op == 0x94 {
				if !p.map32Float(a) || !p.map32Float(b) {
					return false
				}
				id = p.add(regionLoopNode{op: op, left: a, right: b, uses: p.nodes[a].uses | p.nodes[b].uses})
			} else if (op == 0x6a || op == 0x6c) && (p.nodes[a].op != 0 || p.nodes[b].op != 0) {
				if !p.map32Int(a) || !p.map32Int(b) {
					return false
				}
				id = p.add(regionLoopNode{op: op, left: a, right: b, uses: p.nodes[a].uses | p.nodes[b].uses})
			} else if op == 0x47 {
				if p.nodes[a].op != 0 || p.nodes[b].op != 0 {
					return false
				}
				id = p.add(regionLoopNode{op: op, left: a, right: b, uses: p.nodes[a].uses | p.nodes[b].uses})
			} else {
				id = p.integer(op, a, b)
			}
		case 0x45:
			if step != 1 || depth != 1 || p.nodes[stack[0]].op != 0 {
				return false
			}
			id = p.add(regionLoopNode{op: op, left: stack[0]})
			depth--
		case 0x0c:
			if p.headerCount == 0 || imm.Index != 0 || depth != 0 {
				return false
			}
			l := p.locals[p.headerCount-1]
			zero := p.add(regionLoopNode{})
			id = p.add(regionLoopNode{op: 0x47, left: l.value, right: zero, uses: p.nodes[l.value].uses})
			if zero == 0 || id == 0 {
				return false
			}
			end, err := r.Byte()
			if err != nil || end != 0x0b {
				return false
			}
			p.endPC, p.predicate, p.zeroTerminated = start+r.Offset(), id, true
			return p.finish()
		case 0x0d:
			if step == 2 && imm.Index == 1 && depth == 1 && p.nodes[stack[0]].op == 0x45 {
				value := p.nodes[p.nodes[stack[0]].left]
				if value.bits != 0 {
					return false
				}
				found := false
				for i, c := range value.coeff {
					if c != 0 {
						if c != 1 || found {
							return false
						}
						p.headerCount = uint8(i + 1)
						found = true
					}
				}
				if !found {
					return false
				}
				depth--
				continue
			}
			if depth == 1 && p.nodes[stack[0]].op == 0 {
				value := stack[0]
				zero := p.add(regionLoopNode{})
				pred := p.add(regionLoopNode{op: 0x47, left: value, right: zero, uses: p.nodes[value].uses})
				if zero == 0 || pred == 0 {
					return false
				}
				stack[0], p.zeroTerminated = pred, true
			}
			if imm.Index != 0 || depth != 1 || p.storeN == 0 || p.nodes[stack[0]].op != 0x47 {
				return false
			}
			end, err := r.Byte()
			if err != nil || end != 0x0b || r.Offset() > regionLoopMaxBytes {
				return false
			}
			p.endPC, p.predicate = start+r.Offset(), stack[0]
			return p.finish()
		default:
			return false
		}
		if id == 0 || depth == len(stack) {
			return false
		}
		if op != 0x20 {
			p.nodes[id].pos = position
		}
		if op == 0x28 || op == 0x2a || op == 0x43 || id != 0 && p.nodes[id].op != 0 && op != 0x47 && op != 0x20 {
			p.events[p.eventN] = id
			p.eventN++
		}
		stack[depth] = id
		depth++
	}
	return false
}

func (p *regionLoopPlan) stride(id uint8) uint32 {
	var stride uint32
	for i, c := range p.nodes[id].coeff {
		stride += c * p.locals[i].step
	}
	return stride
}

func (p *regionLoopPlan) finish() bool {
	var live uint8
	for _, store := range p.stores[:p.storeN] {
		live |= p.nodes[store.address].uses | p.nodes[store.value].uses
	}
	live |= p.nodes[p.predicate].uses
	for _, l := range p.locals[:p.localN] {
		if l.written {
			live |= p.nodes[l.value].uses
		}
	}
	p.inputMask = live
	var changed uint8
	for i := uint8(0); i < p.localN; i++ {
		l := &p.locals[i]
		if !l.written || live&(1<<i) == 0 || l.typ == mtF32 || p.nodes[l.value].op != 0 {
			continue
		}
		n := p.nodes[l.value]
		if n.uses != 1<<i || n.coeff[i] != 1 || n.bits == 0 || n.bits >= 1<<31 && n.bits != 0xffffffff {
			return false
		}
		l.step = uint32(n.bits)
		changed |= 1 << i
	}
	pred := p.nodes[p.predicate]
	found := false
	for _, edges := range [2][2]uint8{{pred.left, pred.right}, {pred.right, pred.left}} {
		n, limit := p.nodes[edges[0]], p.nodes[edges[1]]
		if limit.uses&changed != 0 {
			continue
		}
		for i := uint8(0); i < p.localN; i++ {
			if p.locals[i].step != 0 && n.uses == 1<<i && n.coeff[i] == 1 && n.bits == uint64(p.locals[i].step) {
				if found {
					return false
				}
				p.counter, p.limit, found = i, edges[1], true
			}
		}
	}
	if !found {
		return false
	}
	step := p.locals[p.counter].step
	if step&(step-1) != 0 && !(p.zeroTerminated && step == 0xffffffff) {
		return false
	}
	if p.stride(p.stores[0].address) >= 1<<31 {
		return false
	}
	for id := uint8(1); id <= p.nodeN; id++ {
		n := p.nodes[id]
		if n.op != 0x28 && n.op != 0x2a {
			continue
		}
		if p.loadN != 0 {
			return false
		}
		stride := p.stride(n.left)
		if stride >= 1<<31 {
			return false
		}
		p.loads[0] = regionLoopLoad{node: id, bytes: 4, stride: stride}
		p.loadN = 1
	}
	return p.loadN == 1
}
