package amd64

// ResetLowestBit emits BMI1 BLSR dst,src. Unlike ordinary NDS VEX forms,
// the destination is encoded in vvvv and ModRM.reg is the fixed /1 digit.
func (a *Asm) ResetLowestBit(dst, src Reg, wide bool) {
	if regallocCheckEnabled {
		a.regallocGPWrite(regallocGPRegMask(dst))
	}
	prefix := byte(0xe2)
	if src >= 8 {
		prefix = 0xc2
	}
	control := ((^byte(dst)) & 15) << 3
	if wide {
		control |= 0x80
	}
	a.emit(0xc4, prefix, control, 0xf3, 0xc8|byte(src&7))
}
