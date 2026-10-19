//go:build amd64

package amd64

import "github.com/wago-org/wago/src/core/compiler/wasm"

// A bounded symbolic proof admits only independent scalar floating-point maps.
// The vector prefix preserves the expression tree and leaves an original scalar
// packet to restore every observable local. All range/alias failures fall back.
type floatMapNode struct {
	op   byte
	a, b int
	bits uint64
}
type floatMapAddress struct {
	base      int
	bias      uint32
	invariant bool
}
type floatMapPlan struct {
	nodes                        [128]floatMapNode
	addresses                    [128]floatMapAddress
	values                       [32]int
	written                      [32]bool
	localType                    [32]machineType
	counter, result, dest, count int
	step, limit                  uint32
	wide                         bool
}

func (f *fn) inspectFloatMapLoop(source *wasm.Reader) (p floatMapPlan, ok bool) {
	if f.nLocals > 32 {
		return
	}
	scan := *source
	start := scan.Offset()
	var stack [24]int
	var initialRead [32]bool
	var stores [2][2]int
	depth, ns, n := 0, 0, f.nLocals
	for i := 0; i < n; i++ {
		p.nodes[i] = floatMapNode{op: 0x20, a: i}
		p.values[i] = i
		p.localType[i] = f.localType[i]
	}
	push := func(x int) bool {
		if depth == len(stack) || x < 0 {
			return false
		}
		stack[depth] = x
		depth++
		return true
	}
	node := func(v floatMapNode) int {
		if n == len(p.nodes) {
			return -1
		}
		p.nodes[n] = v
		n++
		return n - 1
	}
	for scan.Offset()-start < 512 {
		op, err := scan.Byte()
		if err != nil {
			return p, false
		}
		switch op {
		case 0x20, 0x21, 0x22:
			x, e := scan.U32()
			if e != nil || x >= uint32(f.nLocals) {
				return p, false
			}
			if op == 0x20 {
				if p.values[x] == int(x) {
					initialRead[x] = true
				}
				if !push(p.values[x]) {
					return p, false
				}
			} else {
				if depth == 0 {
					return p, false
				}
				p.values[x] = stack[depth-1]
				p.written[x] = true
				if op == 0x21 {
					depth--
				}
			}
		case 0x41, 0x43, 0x44:
			var bits uint64
			if op == 0x41 {
				v, e := scan.I32()
				err = e
				bits = uint64(uint32(v))
			} else if op == 0x43 {
				v, e := scan.LEU32()
				err = e
				bits = uint64(v)
			} else {
				bits, err = scan.LEU64()
			}
			if err != nil || !push(node(floatMapNode{op: op, bits: bits})) {
				return p, false
			}
		case 0x6a, 0x47, 0x92, 0x93, 0x94, 0x95, 0xa0, 0xa1, 0xa2, 0xa3:
			if depth < 2 {
				return p, false
			}
			depth -= 2
			if !push(node(floatMapNode{op: op, a: stack[depth], b: stack[depth+1]})) {
				return p, false
			}
		case 0x2a, 0x2b:
			mem, off, e := f.readMemArg(&scan)
			if e != nil || mem != 0 || off != 0 || depth == 0 {
				return p, false
			}
			x := node(floatMapNode{op: op, a: stack[depth-1]})
			if x < 0 {
				return p, false
			}
			stack[depth-1] = x
		case 0x38, 0x39:
			mem, off, e := f.readMemArg(&scan)
			if e != nil || mem != 0 || off != 0 || depth < 2 || ns == len(stores) {
				return p, false
			}
			depth -= 2
			stores[ns] = [2]int{stack[depth], stack[depth+1]}
			ns++
			if ns == 1 {
				p.wide = op == 0x39
			} else if p.wide != (op == 0x39) {
				return p, false
			}
		case 0x0d:
			label, e := scan.U32()
			if e != nil || label != 0 || depth != 1 || ns == 0 {
				return p, false
			}
			end, e := scan.Byte()
			if e != nil || end != 0x0b {
				return p, false
			}
			increment := stack[0]
			if condition := p.nodes[increment]; condition.op == 0x47 {
				increment = condition.a
				limit := p.nodes[condition.b]
				if limit.op != 0x41 {
					increment = condition.b
					limit = p.nodes[condition.a]
				}
				if limit.op != 0x41 {
					return p, false
				}
				p.limit = uint32(limit.bits)
			}
			inc := p.nodes[increment]
			if inc.op != 0x6a {
				return p, false
			}
			local, stride := p.nodes[inc.a], p.nodes[inc.b]
			if local.op != 0x20 {
				local, stride = stride, local
			}
			size := uint32(4)
			if p.wide {
				size = 8
			}
			if local.op != 0x20 || f.localType[local.a] != mtI32 || stride.op != 0x41 || stride.bits != uint64(size)*uint64(ns) {
				return p, false
			}
			p.counter, p.step = local.a, uint32(stride.bits)
			if p.values[p.counter] != increment {
				return p, false
			}
			for i := 0; i < f.nLocals; i++ {
				if p.written[i] && initialRead[i] && i != p.counter {
					return p, false
				}
			}
			p.count, p.dest, p.result = n, stores[0][0], stores[0][1]
			if !p.proveAddresses(f.localType) {
				return p, false
			}
			if ns == 2 {
				visits := 0
				a, valid := p.affine(stores[1][0], f.localType, &visits)
				if !valid {
					return p, false
				}
				p.addresses[stores[1][0]] = a
			}
			var used [128]bool
			var mark func(int)
			mark = func(x int) {
				if used[x] {
					return
				}
				used[x] = true
				v := p.nodes[x]
				if v.op == 0x92 || v.op == 0x93 || v.op == 0x94 || v.op == 0x95 || v.op == 0xa0 || v.op == 0xa1 || v.op == 0xa2 || v.op == 0xa3 {
					mark(v.a)
					mark(v.b)
				}
			}
			for i := 0; i < ns; i++ {
				mark(stores[i][1])
			}
			for i := 0; i < n; i++ {
				if (p.nodes[i].op == 0x2a || p.nodes[i].op == 0x2b) && !used[i] {
					return p, false
				}
			}
			if ns == 2 && (!p.sameMap(stores[0][1], stores[1][1], size, 0) || !p.nextAddress(stores[0][0], stores[1][0], size)) {
				return p, false
			}
			return p, p.validExpression(p.result, 0)
		default:
			return p, false
		}
	}
	return p, false
}

func (p *floatMapPlan) affine(x int, types []machineType, visits *int) (floatMapAddress, bool) {
	(*visits)++
	if *visits > 128 {
		return floatMapAddress{}, false
	}
	v := p.nodes[x]
	if v.op == 0x20 && types[v.a] == mtI32 && v.a != p.counter && !p.written[v.a] {
		return floatMapAddress{base: v.a, invariant: true}, true
	}
	if v.op != 0x6a {
		return floatMapAddress{}, false
	}
	// Flatten addition modulo 2^32. Exactly one invariant base and one counter
	// are required; the runtime guard excludes wrap across the entire prefix.
	var base, counter, n int
	var bias uint32
	var walk func(int) bool
	walk = func(x int) bool {
		n++
		if n > 128 {
			return false
		}
		v := p.nodes[x]
		switch v.op {
		case 0x41:
			bias += uint32(v.bits)
		case 0x20:
			if types[v.a] != mtI32 {
				return false
			}
			if v.a == p.counter {
				counter++
			} else {
				if base != 0 || p.written[v.a] {
					return false
				}
				base = v.a + 1
			}
		case 0x6a:
			return walk(v.a) && walk(v.b)
		default:
			return false
		}
		return true
	}
	if !walk(x) || base == 0 || counter > 1 || bias > 1<<20 {
		return floatMapAddress{}, false
	}
	return floatMapAddress{base: base - 1, bias: bias, invariant: counter == 0}, true
}
func (p *floatMapPlan) proveAddresses(types []machineType) bool {
	visits := 0
	addr, ok := p.affine(p.dest, types, &visits)
	if !ok || addr.invariant {
		return false
	}
	p.addresses[p.dest] = addr
	for i := 0; i < p.count; i++ {
		if p.nodes[i].op == 0x2a || p.nodes[i].op == 0x2b {
			visits = 0
			a, ok := p.affine(p.nodes[i].a, types, &visits)
			if !ok {
				return false
			}
			p.addresses[i] = a
		}
	}
	return true
}
func (p *floatMapPlan) validExpression(x, depth int) bool {
	if depth > 8 {
		return false
	}
	v := p.nodes[x]
	if v.op == 0x20 {
		return !p.written[v.a] && p.localType[v.a] == mapFloatType(p.wide)
	}
	if v.op == 0x2b || v.op == 0x44 {
		return p.wide
	}
	if v.op == 0x2a || v.op == 0x43 {
		return !p.wide
	}
	if p.wide && v.op >= 0xa0 && v.op <= 0xa3 || !p.wide && v.op >= 0x92 && v.op <= 0x95 {
		return p.validExpression(v.a, depth+1) && p.validExpression(v.b, depth+1)
	}
	return false
}
func (p *floatMapPlan) nextAddress(x, y int, step uint32) bool {
	a, b := p.addresses[x], p.addresses[y]
	if a.invariant || b.invariant {
		return a.invariant && b.invariant && a.base == b.base && a.bias == b.bias
	}
	return a.base == b.base && uint64(a.bias)+uint64(step) == uint64(b.bias)
}
func (p *floatMapPlan) sameMap(x, y int, step uint32, depth int) bool {
	if depth > 8 {
		return false
	}
	a, b := p.nodes[x], p.nodes[y]
	if a.op != b.op {
		return false
	}
	if a.op == 0x2a || a.op == 0x2b {
		return p.nextAddress(x, y, step)
	}
	if a.op == 0x20 {
		return a.a == b.a
	}
	if a.op == 0x43 || a.op == 0x44 {
		return a.bits == b.bits
	}
	return p.sameMap(a.a, b.a, step, depth+1) && p.sameMap(a.b, b.b, step, depth+1)
}

func mapFloatType(wide bool) machineType {
	if wide {
		return mtF64
	}
	return mtF32
}
