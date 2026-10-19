//go:build amd64

package amd64

import "os"

var rounddownDivEnabled = os.Getenv("WAGO_AMD64_ROUNDDOWN_DIV") == "1"
var canonicalRounddownEnabled = os.Getenv("WAGO_AMD64_CANONICAL_ROUNDDOWN") == "1"

func (f *fn) canonicalRounddownInput(e *elem) bool {
	if !f.opt(optCanonicalRounddown) || e == nil || e.st.typ != mtI32 {
		return false
	}
	if f.opt(optValueFacts) && e.st.valueFacts().has(factUpper32Zero) {
		return true
	}
	if !e.isValue() {
		return false
	}
	switch e.st.kind {
	case stConst, stLocalRef:
		return true
	case stLocalReg, stSlot:
		return f.opt(optCanonicalI32)
	}
	return false
}

// For n=q*d+r and m*d=2^s-e, m*(n+1)=q*2^s+m*(r+1)-q*e.
// With e>0 and m>=floor(UINT32_MAX/d)*e, the final term lies in
// [0,2^s), proving the quotient for every unsigned i32 dividend.
func rounddownReciprocal(d uint64) (uint32, byte, bool) {
	if d < 3 || d > 0xffffffff || d&(d-1) == 0 {
		return 0, 0, false
	}
	for s := byte(32); s < 64; s++ {
		p := uint64(1) << s
		m, e := p/d, p%d
		if m > 0xffffffff {
			break
		}
		if e != 0 && m >= (uint64(0xffffffff)/d)*e {
			return uint32(m), s, true
		}
	}
	return 0, 0, false
}

func (f *fn) tryRounddownDiv(res Reg, d uint64, rem bool, canonical ...bool) bool {
	if !f.opt(optRounddownDiv) {
		return false
	}
	m, s, ok := rounddownReciprocal(d)
	if !ok {
		return false
	}
	q := res
	if rem {
		q = f.allocReg(maskOf(res))
		f.pinned = f.pinned.add(q)
	}
	if len(canonical) != 0 && canonical[0] {
		if q == res {
			f.a.AluRI(0, q, 1, true)
		} else {
			f.a.LeaDisp(q, res, 1)
		}
		f.stats.peep("canonical-rounddown")
	} else {
		f.a.MovRegReg32(q, res)
		f.a.AluRI(0, q, 1, true) // n+1 is 33 bits at UINT32_MAX; never wrap it to zero.
	}
	if m <= 0x7fffffff {
		f.a.ImulRI(q, int32(m), true)
	} else {
		t := f.allocReg(maskOf(res, q))
		f.a.MovImm32(t, int32(m))
		f.a.IMul(q, t, true)
		f.release(t)
	}
	f.a.ShiftImm(5, q, s, true)
	if rem {
		f.remFromQuot(res, q, int64(d), false, false)
		f.pinned = f.pinned.remove(q)
	}
	f.stats.peep("rounddown-div")
	return true
}
