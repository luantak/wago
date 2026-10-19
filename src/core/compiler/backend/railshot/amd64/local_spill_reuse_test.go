//go:build amd64

package amd64

import (
	encoder "github.com/wago-org/wago/src/core/encoder/amd64"
	"testing"
)

func TestLocalSpillReuse(t *testing.T) {
	saved := localSpillReuseEnabled
	localSpillReuseEnabled = true
	defer func() { localSpillReuseEnabled = saved }()
	fixture := func() (*fn, *elem) {
		f := &fn{a: &encoder.Asm{}, s: newStackWithCap(8), stats: new(CodegenStats), policy: currentCodegenPolicy(),
			localType: []machineType{mtI32}, locals: []localDef{{typ: mtI32, reg: regNone, state: lsMem}}}
		e := f.pushReg(RAX, mtI32)
		e.st.cval = 1
		return f, e
	}
	f, e := fixture()
	for _, r := range gpAlloc {
		if r != RAX {
			f.reserved = f.reserved.add(r)
		}
	}
	if r := f.allocReg(0); r != RAX {
		t.Fatalf("allocated %v", r)
	}
	if e.st.kind != stLocalRef || e.st.idx != 0 || len(f.a.B) != 0 || f.regUser[RAX] != nil {
		t.Fatalf("spill not reused: %+v", e.st)
	}
	for _, invalidate := range []string{"write", "boundary", "consume", "sink", "dirty"} {
		f, e = fixture()
		switch invalidate {
		case "write":
			f.forgetLocalSpillCopies(0)
		case "boundary":
			f.forgetLocalSpillCopies(-1)
		case "consume":
			f.materialize(e)
		case "sink":
			f.condenseInto(e, RAX)
		case "dirty":
			f.markLocalDirty(0)
		}
		if f.reuseLocalSpill(e) {
			t.Fatalf("stale copy after %s", invalidate)
		}
	}
}
