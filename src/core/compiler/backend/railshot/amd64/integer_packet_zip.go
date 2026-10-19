//go:build amd64

package amd64

func zipIntegerPacket(nodes []integerPacketNode, written []bool, terms [32]int, nt int, p integerPacketPlan, variants bool) (integerPacketPlan, bool) {
	index := p.index
	visits := 0
	var usedLoads [512]bool
	var order [32]int
	ordered := false
	add := func(v integerPacketNode) int {
		for i := 0; i < p.n; i++ {
			if p.nodes[i] == v {
				return i
			}
		}
		if p.n == len(p.nodes) {
			return -1
		}
		p.nodes[p.n] = v
		p.n++
		return p.n - 1
	}
	addLanes := func(inputs [32]integerPacketNode) int {
		if !variants {
			return -1
		}
		for i := 0; i < p.n; i++ {
			if p.nodes[i].op == 0xfd && p.laneInputs[i] == inputs {
				return i
			}
		}
		if p.n == len(p.nodes) {
			return -1
		}
		id := p.n
		if p.laneInputs == nil {
			p.laneInputs = new([16][32]integerPacketNode)
		}
		p.n++
		p.nodes[id] = integerPacketNode{op: 0xfd}
		p.laneInputs[id] = inputs
		return id
	}
	visits = 0
	var zip func([32]int, int) int
	zip = func(xs [32]int, depth int) int {
		visits++
		if depth > 16 || visits > 512 {
			return -1
		}
		v := nodes[xs[0]]
		for lane := 1; lane < nt; lane++ {
			if nodes[xs[lane]].op != v.op {
				return -1
			}
		}
		switch v.op {
		case 0x20:
			var inputs [32]integerPacketNode
			uniform := true
			for lane := 0; lane < nt; lane++ {
				inputs[lane] = nodes[xs[lane]]
				if written[inputs[lane].a] {
					return -1
				}
				uniform = uniform && inputs[lane].a == v.a
			}
			if !uniform {
				return addLanes(inputs)
			}
			return add(v)
		case 0x41:
			var inputs [32]integerPacketNode
			uniform := true
			for lane := 0; lane < nt; lane++ {
				inputs[lane] = nodes[xs[lane]]
				uniform = uniform && inputs[lane].v == v.v
			}
			if !uniform {
				return addLanes(inputs)
			}
			return add(v)
		case 0x28:
			var offsets [32]uint32
			base := -2
			for lane := 0; lane < nt; lane++ {
				b, off, ok := affineIntegerPacketProgramAddress(nodes, nodes[xs[lane]].a, index, variants)
				if !ok || b >= 0 && written[b] || (base != -2 && base != b) {
					return -1
				}
				base = b
				offsets[lane] = off
				usedLoads[xs[lane]] = true
			}
			if !ordered {
				for lane := 0; lane < nt; lane++ {
					order[lane] = lane
				}
				for i := 0; i < nt; i++ {
					for j := i + 1; j < nt; j++ {
						if offsets[order[j]] < offsets[order[i]] {
							order[i], order[j] = order[j], order[i]
						}
					}
				}
				ordered = true
				for lane := 0; lane < nt; lane++ {
					p.laneOrder[lane] = uint8(order[lane])
				}
			}
			minimum := offsets[order[0]]
			for lane := 0; lane < nt; lane++ {
				if uint64(offsets[order[lane]]) != uint64(minimum)+uint64(lane*4) {
					return -1
				}
			}
			return add(integerPacketNode{op: 0x28, a: base, v: minimum})
		case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76:
			var left, right [32]int
			for lane := 0; lane < nt; lane++ {
				left[lane] = nodes[xs[lane]].a
				right[lane] = nodes[xs[lane]].b
			}
			if (v.op == 0x74 || v.op == 0x75 || v.op == 0x76) && nodes[right[0]].op != 0x41 {
				return -1
			}
			a, b := zip(left, depth+1), zip(right, depth+1)
			if a < 0 || b < 0 {
				return -1
			}
			if (v.op == 0x74 || v.op == 0x75 || v.op == 0x76) && p.nodes[b].op != 0x41 {
				return -1
			}
			return add(integerPacketNode{op: v.op, a: a, b: b})
		}
		return -1
	}
	p.root = zip(terms, 0)
	valid := p.root >= 0 && ordered
	for x, v := range nodes {
		if v.op == 0x28 && !usedLoads[x] {
			valid = false
		}
	}
	return p, valid
}
