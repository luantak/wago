//go:build linux && amd64

package amd64

import "testing"

func TestBoundedRemainderLea(t *testing.T) {
	saved := boundedRemainderLeaEnabled
	boundedRemainderLeaEnabled = true
	defer func() { boundedRemainderLeaEnabled = saved }()
	TestBoundedUnsignedDivNative(t)
}
