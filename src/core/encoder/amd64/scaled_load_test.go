package amd64

import (
	"bytes"
	"testing"
)

func TestLoadSigned32Scaled(t *testing.T) {
	for _, tc := range []struct {
		dst, base, index Reg
		scale            byte
		disp             int32
		want             []byte
	}{
		{RDX, RAX, RDI, 2, 0, []byte{0x48, 0x63, 0x14, 0xb8}},
		{R10, R13, R9, 3, 128, []byte{0x4f, 0x63, 0x94, 0xcd, 0x80, 0, 0, 0}},
		{RDX, RBP, RDI, 2, 0, []byte{0x48, 0x63, 0x54, 0xbd, 0}},
	} {
		var a Asm
		a.LoadSigned32Scaled(tc.dst, tc.base, tc.index, tc.scale, tc.disp)
		if !bytes.Equal(a.B, tc.want) {
			t.Fatalf("got=%x want=%x", a.B, tc.want)
		}
	}
}
