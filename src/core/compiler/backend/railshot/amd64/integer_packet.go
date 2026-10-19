//go:build amd64

package amd64

import "github.com/wago-org/wago/src/core/compiler/wasm"

type integerPacketNode struct {
	op   byte
	a, b int
	v    uint32
}

type integerPacketPlan struct {
	counter, index, acc, lanes int
	stride                     integerPacketNode
	nodes                      [16]integerPacketNode
	n, root                    int
	laneInputs                 *[16][32]integerPacketNode
	laneOrder                  [32]uint8
	post                       [16]integerPacketPost
	postN                      int
	program                    bool
}

type integerPacketPost struct {
	op      byte
	reverse bool
	operand integerPacketNode
}

func (f *fn) inspectIntegerPacket(source *wasm.Reader) (p integerPacketPlan, ok bool) {
	program := f.opt(optIntegerPacketProgram)
	localLimit, nodeLimit, byteLimit := 32, 256, 1024
	if program {
		localLimit, nodeLimit, byteLimit = 64, 512, 2048
	}
	if f.nLocals > localLimit || len(f.localType) != f.nLocals {
		return p, false
	}
	for _, typ := range f.localType {
		if typ != mtI32 {
			return p, false
		}
	}
	var nodes [512]integerPacketNode
	var values [64]int
	var written, initialRead [64]bool
	var stack [96]int
	n, depth := f.nLocals, 0
	for i := 0; i < n; i++ {
		nodes[i] = integerPacketNode{op: 0x20, a: i}
		values[i] = i
	}
	makeNode := func(v integerPacketNode) int {
		if n == nodeLimit {
			return -1
		}
		nodes[n] = v
		n++
		return n - 1
	}
	push := func(x int) bool {
		if x < 0 || depth == len(stack) {
			return false
		}
		stack[depth] = x
		depth++
		return true
	}
	scan := *source
	start := scan.Offset()
	for scan.Offset()-start < byteLimit {
		op, e := scan.Byte()
		if e != nil {
			return p, false
		}
		switch op {
		case 0x20, 0x21, 0x22:
			x, e := scan.U32()
			if e != nil || x >= uint32(f.nLocals) {
				return p, false
			}
			if op == 0x20 {
				if values[x] == int(x) {
					initialRead[x] = true
				}
				if !push(values[x]) {
					return p, false
				}
			} else {
				if depth == 0 {
					return p, false
				}
				values[x] = stack[depth-1]
				written[x] = true
				if op == 0x21 {
					depth--
				}
			}
		case 0x41:
			v, e := scan.I32()
			if e != nil || !push(makeNode(integerPacketNode{op: op, v: uint32(v)})) {
				return p, false
			}
		case 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76:
			if (op == 0x6d || op == 0x6e) && !program {
				return p, false
			}
			if depth < 2 {
				return p, false
			}
			depth -= 2
			if !push(makeNode(integerPacketNode{op: op, a: stack[depth], b: stack[depth+1]})) {
				return p, false
			}
		case 0x28:
			mem, off, e := f.readMemArg(&scan)
			if e != nil || mem != 0 || off != 0 || depth == 0 {
				return p, false
			}
			x := makeNode(integerPacketNode{op: op, a: stack[depth-1]})
			if x < 0 {
				return p, false
			}
			stack[depth-1] = x
		case 0x0d:
			label, e := scan.U32()
			if e != nil || label != 0 || depth != 1 {
				return p, false
			}
			end, e := scan.Byte()
			if e != nil || end != 0x0b {
				return p, false
			}
			inc := nodes[stack[0]]
			if inc.op != 0x6a {
				return p, false
			}
			local, step := nodes[inc.a], nodes[inc.b]
			if local.op != 0x20 {
				local, step = step, local
			}
			if local.op != 0x20 || step.op != 0x41 || step.v != 0xffffffff || values[local.a] != stack[0] {
				return p, false
			}
			p.counter = local.a
			if program {
				return proveIntegerPacketProgram(nodes[:n], values[:f.nLocals], written[:f.nLocals], initialRead[:f.nLocals], p)
			}
			return proveIntegerPacket(nodes[:n], values[:f.nLocals], written[:f.nLocals], initialRead[:f.nLocals], p)
		default:
			return p, false
		}
	}
	return p, false
}

func affineIntegerPacketAddress(nodes []integerPacketNode, x int, counter int) (base int, offset uint32, ok bool) {
	return affineIntegerPacketProgramAddress(nodes, x, counter, false)
}

func affineIntegerPacketProgramAddress(nodes []integerPacketNode, x int, counter int, absolute bool) (base int, offset uint32, ok bool) {
	var locals [2]int
	n := 0
	visits := 0
	var visit func(int) bool
	visit = func(x int) bool {
		visits++
		if visits > 128 {
			return false
		}
		v := nodes[x]
		switch v.op {
		case 0x20:
			if n == 2 {
				return false
			}
			locals[n] = v.a
			n++
			return true
		case 0x41:
			if v.v > 1<<20 || uint64(offset)+uint64(v.v) > 1<<20 {
				return false
			}
			offset += v.v
			return true
		case 0x6a:
			return visit(v.a) && visit(v.b)
		}
		return false
	}
	if !visit(x) {
		return 0, 0, false
	}
	if absolute && n == 1 && locals[0] == counter {
		return -1, offset, true
	}
	if n != 2 || locals[0] == locals[1] {
		return 0, 0, false
	}
	if locals[0] == counter {
		return locals[1], offset, true
	}
	if locals[1] == counter {
		return locals[0], offset, true
	}
	return 0, 0, false
}

// Zip equal expression operators rather than recognizing a reduction kernel.
// Only addition is reassociated: all lane operations keep their source tree.
func proveIntegerPacket(nodes []integerPacketNode, values []int, written, initialRead []bool, seed integerPacketPlan) (integerPacketPlan, bool) {
	for acc := range values {
		if !written[acc] || !initialRead[acc] || acc == seed.counter {
			continue
		}
		var terms [32]int
		nt, reads, visits := 0, 0, 0
		var sum func(int) bool
		sum = func(x int) bool {
			visits++
			if visits > 512 {
				return false
			}
			v := nodes[x]
			if v.op == 0x20 && v.a == acc {
				reads++
				return true
			}
			if v.op == 0x6a {
				return sum(v.a) && sum(v.b)
			}
			if nt == 8 {
				return false
			}
			terms[nt] = x
			nt++
			return true
		}
		if !sum(values[acc]) || reads != 1 || (nt != 4 && nt != 8) {
			continue
		}
		for index := range values {
			if index == acc || index == seed.counter || !written[index] || !initialRead[index] {
				continue
			}
			advance := nodes[values[index]]
			if advance.op != 0x6a {
				continue
			}
			idx, stride := nodes[advance.a], nodes[advance.b]
			if idx.op != 0x20 || idx.a != index {
				idx, stride = stride, idx
			}
			if idx.op != 0x20 || idx.a != index || (stride.op != 0x41 && (stride.op != 0x20 || written[stride.a])) {
				continue
			}
			carried := false
			for i := range values {
				if written[i] && initialRead[i] && i != acc && i != index && i != seed.counter {
					carried = true
				}
			}
			if carried {
				continue
			}
			p := seed
			p.index = index
			p.acc = acc
			p.lanes = nt
			p.stride = stride
			if result, ok := zipIntegerPacket(nodes, written, terms, nt, p, false); ok {
				return result, true
			}
		}
	}
	return seed, false
}
