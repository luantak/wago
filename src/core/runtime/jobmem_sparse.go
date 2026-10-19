//go:build (linux || darwin || windows) && (amd64 || arm64)

package runtime

import "os"

var sparseJobMemoryReuseEnabled = os.Getenv("WAGO_SPARSE_JOB_MEMORY_REUSE") == "1"

// A platform may zero a small resident working set without discarding its
// backing pages. Returning false requires the ordinary whole-range fallback.
var sparseJobMemoryReclaim func([]byte) bool
