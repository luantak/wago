//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var wrappedLoadGroupEnabled = os.Getenv("WAGO_AMD64_WRAPPED_LOAD_GROUP") == "1"
var signedLoadGroupEnabled = os.Getenv("WAGO_AMD64_SIGNED_LOAD_GROUP") == "1"

type wrappedLoadGroup struct {
	valid, check       bool
	local              uint32
	current, low, high int32
}

func (f *fn) wrappedLocalAddress(e *elem) (uint32, int32, bool) {
	c := int32(0)
	if e != nil && e.isDeferred() && e.deferredOp() == opAdd && e.arg1.isValue() && e.arg1.st.kind == stConst &&
		(e.arg1.st.cval >= 0 || f.opt(optSignedLoadGroup)) && e.arg1.st.cval >= -0x80000000 && e.arg1.st.cval <= 0x7fffffff {
		c = int32(e.arg1.st.cval)
		e = e.arg0
	}
	if e != nil && e.isValue() && e.valueType() == mtI32 && (e.st.kind == stLocalRef || e.st.kind == stLocalReg) {
		return e.st.idx, c, true
	}
	return 0, 0, false
}

func (f *fn) inspectWrappedLoadGroup(r *wasm.Reader, off uint32, size int) (p wrappedLoadGroup) {
	if !f.opt(optWrappedLoadGroup) || !f.boundsFacts || f.guardMode || f.threadedMemory0 || f.localBase != 0 {
		return
	}
	x, c, ok := f.wrappedLocalAddress(f.s.back())
	end := int64(c) + int64(off) + int64(size)
	if !ok || end > 0x7fffffff {
		return
	}
	p = wrappedLoadGroup{valid: true, local: x, current: c, low: c, high: int32(end)}
	if f.wrappedGroupCovers(p) {
		return
	}
	mt, ok := f.memoryType(0)
	if !ok || !mt.Limits.HasMax || mt.Limits.Addr64 {
		return
	}
	scan := *r
	state, index, add := byte(0), uint32(0), int32(0)
	count := 1
	hasArithmetic := c != 0
	for n := 0; n < maxStraightLineRangeOps && scan.HasNext(); n++ {
		op, err := scan.Byte()
		if err != nil {
			break
		}
		next := byte(0)
		switch {
		case op == 0x20:
			index, err = scan.U32()
			if err != nil {
				return p
			}
			add = 0
			next = 1
		case op == 0x41:
			v, e := scan.I32()
			if e != nil {
				return p
			}
			if state == 1 && (v >= 0 || f.opt(optSignedLoadGroup)) {
				add = v
				next = 2
			}
		case op == 0x6a:
			if state == 2 {
				next = 3
			}
		case op == 0x21 || op == 0x22:
			i, e := scan.U32()
			if e != nil || i == x {
				goto done
			}
		case op >= 0x28 && op <= 0x35:
			align, e := scan.U32()
			if e != nil || align >= 64 {
				goto done
			}
			o, e := scan.U32()
			if e != nil {
				goto done
			}
			if (state == 1 || state == 3) && index == x {
				hasArithmetic = hasArithmetic || add != 0
				end := int64(add) + int64(o) + int64(memAccessSize(op))
				if end > 0x7fffffff {
					goto done
				}
				if add < p.low {
					p.low = add
				}
				if int32(end) > p.high {
					p.high = int32(end)
				}
				count++
			}
		case op == 0x23:
			if _, e := scan.U32(); e != nil {
				goto done
			}
		case op == 0x01 || op == 0x1a || op == 0x1b || op == 0x42 || op >= 0x45 && op <= 0x6c || op >= 0x71 && op <= 0x7e || op >= 0x83 && op <= 0x8a || op == 0xa7 || op == 0xac || op == 0xad:
			if skipImmediates(&scan, op) != nil {
				goto done
			}
		default:
			goto done
		}
		state = next
	}
done:
	// Let a=(base+low) mod 2^32, span=high-low. If [a,a+span)
	// crosses 2^32, a > 2^32-span >= declared maximum bytes, so
	// the load at low already traps. Otherwise all selected loads are
	// valid exactly when a+span <= current memory bytes. This also accepts
	// the case where every address addition wraps into low memory.
	// Signed endpoints must be subtracted at native width. The emitted LEA
	// uses a positive signed displacement, so decline wider modular intervals.
	width := int64(p.high) - int64(p.low)
	if width <= 0 || width > 0x7fffffff {
		return p
	}
	span := uint64(width)
	// For negative additions, require the first checked address to be the
	// interval's low endpoint. Otherwise constructing a new modular base adds
	// work to the first access and can lengthen the hot address dependency.
	if p.low < 0 && p.current != p.low {
		return p
	}
	p.check = hasArithmetic && count >= 2 && mt.Limits.Max <= ((uint64(1)<<32)-span)>>16
	return
}

func (f *fn) wrappedGroupCovers(p wrappedLoadGroup) bool {
	for _, c := range f.boundsCerts {
		if c.kind == 3 && c.idx == p.local && p.current >= c.wrappedLow && p.high <= c.extent {
			return true
		}
	}
	return false
}

func (f *fn) emitWrappedLoadGroup(p wrappedLoadGroup, ea Reg) bool {
	if !p.valid {
		return false
	}
	if f.wrappedGroupCovers(p) {
		f.stats.addBoundsElidable()
		return true
	}
	if !p.check {
		return false
	}
	f.pinned = f.pinned.add(ea)
	t := f.allocReg(0)
	if p.low < 0 && p.low == p.current {
		// memAddr has already canonicalized ea to an unsigned memory32 address.
		f.a.LeaDisp(t, ea, p.high-p.low)
	} else {
		f.a.LeaDispW(t, ea, p.low-p.current, false)
		f.a.LeaDisp(t, t, p.high-p.low)
	}
	if f.memSizeReg != regNone {
		f.a.Cmp64(t, f.memSizeReg)
	} else {
		f.a.AluRM(cmpRMcode, t, RBX, -bdCurBytes, true)
	}
	f.trapIf(condA, trapMemOOB)
	f.release(t)
	f.pinned = f.pinned.remove(ea)
	slot := -1
	for i, c := range f.boundsCerts {
		if c.kind == 3 && c.idx == p.local {
			slot = i
			break
		}
		if slot < 0 && c.kind == 0 {
			slot = i
		}
	}
	if slot < 0 {
		slot = int(f.nextBoundsCert) % len(f.boundsCerts)
		f.nextBoundsCert++
	}
	f.boundsCerts[slot] = boundsCert{kind: 3, idx: p.local, extent: p.high, wrappedLow: p.low}
	f.stats.peep("wrapped-load-group")
	return true
}
