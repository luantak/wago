//go:build amd64

package amd64

import "os"

var integerPacketProgramEnabled = os.Getenv("WAGO_AMD64_INTEGER_PACKET_PROGRAM") == "1"
var packetInputHoistEnabled = os.Getenv("WAGO_AMD64_INTEGER_PACKET_INPUT_HOIST") == "1"

// The packet may feed a bounded chain of total scalar operations. The chain
// preserves operand order; only modular integer addition inside the packet is
// reassociated. Trapping division and arbitrary scalar subtrees are rejected.
func integerPacketPostPath(nodes []integerPacketNode, root, stop int) ([16]integerPacketPost, int, bool) {
	var steps [16]integerPacketPost
	n := 0
	ready := func(x int) bool { return nodes[x].op == 0x20 || nodes[x].op == 0x41 }
	var path func(int, int) bool
	path = func(x, depth int) bool {
		if x == stop {
			return true
		}
		if depth == 16 {
			return false
		}
		v := nodes[x]
		switch v.op {
		case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73:
			if ready(v.b) && path(v.a, depth+1) {
				steps[n] = integerPacketPost{op: v.op, operand: nodes[v.b]}
				n++
				return true
			}
			if ready(v.a) && path(v.b, depth+1) {
				steps[n] = integerPacketPost{op: v.op, reverse: true, operand: nodes[v.a]}
				n++
				return true
			}
		case 0x6d, 0x6e, 0x74, 0x75, 0x76:
			c := nodes[v.b]
			if c.op != 0x41 {
				return false
			}
			if v.op == 0x6d || v.op == 0x6e {
				if c.v == 0 || c.v&(c.v-1) != 0 || v.op == 0x6d && c.v >= 0x80000000 {
					return false
				}
			}
			if path(v.a, depth+1) {
				steps[n] = integerPacketPost{op: v.op, operand: c}
				n++
				return true
			}
		}
		return false
	}
	ok := path(root, 0)
	return steps, n, ok
}

func proveIntegerPacketProgram(nodes []integerPacketNode, values []int, written, initialRead []bool, seed integerPacketPlan) (integerPacketPlan, bool) {
	// Keep the retained packet path byte-identical when the extended grammar
	// provides no additional opportunity.
	if len(nodes) <= 256 && len(values) <= 32 {
		if p, ok := proveIntegerPacket(nodes, values, written, initialRead, seed); ok {
			return p, true
		}
	}
	for acc := range values {
		if !written[acc] || !initialRead[acc] || acc == seed.counter {
			continue
		}
		for candidate := -1; candidate < len(nodes); candidate++ {
			root := values[acc]
			if candidate >= 0 {
				root = candidate
				if nodes[root].op != 0x6a || root == values[acc] {
					continue
				}
			}
			var terms [32]int
			nt, reads, visits := 0, 0, 0
			var collect func(int) bool
			collect = func(x int) bool {
				visits++
				if visits > 1024 {
					return false
				}
				v := nodes[x]
				if candidate == -1 && v.op == 0x20 && v.a == acc {
					reads++
					return true
				}
				if v.op == 0x6a {
					return collect(v.a) && collect(v.b)
				}
				if nt == 32 {
					return false
				}
				terms[nt] = x
				nt++
				return true
			}
			if !collect(root) || (nt != 4 && nt != 8 && nt != 16 && nt != 32) || reads > 1 {
				continue
			}
			var steps [16]integerPacketPost
			ns := 0
			if candidate == -1 {
				if reads == 1 {
					steps[0] = integerPacketPost{op: 0x6a, operand: integerPacketNode{op: 0x20, a: acc}}
					ns = 1
				}
			} else {
				var ok bool
				steps, ns, ok = integerPacketPostPath(nodes, values[acc], root)
				if !ok {
					continue
				}
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
				valid := true
				for i := range values {
					if written[i] && initialRead[i] && i != acc && i != index && i != seed.counter {
						valid = false
					}
				}
				for _, s := range steps[:ns] {
					if s.operand.op == 0x20 {
						i := s.operand.a
						if written[i] && i != acc && i != index && i != seed.counter {
							valid = false
						}
					}
				}
				if !valid {
					continue
				}
				p := seed
				p.acc = acc
				p.index = index
				p.stride = stride
				p.lanes = nt
				p.program = true
				p.post = steps
				p.postN = ns
				if result, ok := zipIntegerPacket(nodes, written, terms, nt, p, true); ok {
					return result, true
				}
			}
		}
	}
	return seed, false
}
