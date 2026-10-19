//go:build amd64

package amd64

import "os"

var scaledJumpTableEnabled = os.Getenv("WAGO_AMD64_SCALED_JUMP_TABLE") == "1"
