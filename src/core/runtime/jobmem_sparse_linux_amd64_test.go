//go:build linux && amd64 && !tinygo

package runtime

import "testing"

func TestSparseJobMemoryKeepsResidentPages(t *testing.T) {
	saved := sparseJobMemoryReuseEnabled
	sparseJobMemoryReuseEnabled = true
	defer func() { sparseJobMemoryReuseEnabled = saved }()
	j, err := NewJobMemoryGrowable(2<<20, 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for _, off := range []int{0, 8 * pageSize, 16 * pageSize} {
		j.LinearMemory()[off] = 0xa5
	}
	b := j.mem[:roundUpPage(basedataSize+j.curBytes())]
	before, after := make([]byte, len(b)/pageSize), make([]byte, len(b)/pageSize)
	if err := jobMemoryResidency(b, before); err != nil {
		t.Fatal(err)
	}
	if err := j.reclaimForReuse(); err != nil {
		t.Fatal(err)
	}
	if err := jobMemoryResidency(b, after); err != nil {
		t.Fatal(err)
	}
	for i, v := range before {
		if v&1 != 0 && after[i]&1 == 0 {
			t.Fatalf("resident page%d was discarded", i)
		}
	}
	for i, v := range b {
		if v != 0 {
			t.Fatalf("byte%d leaked value%x", i, v)
		}
	}
}

func TestSparseJobMemoryStaleSnapshotZeroesAll(t *testing.T) {
	b, err := mmapRW(16 * pageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer munmap(b)
	for _, pattern := range []byte{0, 1, 0xfe, 0xff, 2} {
		for i := range b {
			b[i] = 0xa5
		}
		vec := make([]byte, 16)
		for i := range vec {
			vec[i] = pattern
			if pattern == 2 {
				vec[i] = byte(i & 1)
			}
		}
		// Classify resident dirty pages as absent too. Discarding the absent
		// spans must erase their old data, not merely skip those pages.
		if !zeroSparseJobMemorySnapshot(b, vec) {
			t.Fatal("snapshot zeroing failed")
		}
		for i, v := range b {
			if v != 0 {
				t.Fatalf("pattern=%x byte%d leaked%x", pattern, i, v)
			}
		}
	}
}

func TestSparseJobMemoryBoundedFallback(t *testing.T) {
	saved := sparseJobMemoryReuseEnabled
	sparseJobMemoryReuseEnabled = true
	defer func() { sparseJobMemoryReuseEnabled = saved }()
	for _, shape := range []string{"dense", "fragmented", "large"} {
		n := 2 << 20
		if shape == "large" {
			n = 8 << 20
		}
		j, err := NewJobMemoryGrowable(n, n)
		if err != nil {
			t.Fatal(err)
		}
		lin := j.LinearMemory()
		switch shape {
		case "dense":
			for i := 0; i < len(lin); i += pageSize {
				lin[i] = 0xa5
			}
		case "fragmented":
			for i := 0; i < 12; i++ {
				lin[(2*i+1)*pageSize] = 0xa5
			}
		case "large":
			lin[len(lin)-1] = 0xa5
		}
		b := j.mem[:roundUpPage(basedataSize+j.curBytes())]
		if reclaimSparseJobMemory(b) {
			t.Fatalf("missed %s budget", shape)
		}
		if err := j.reclaimForReuse(); err != nil {
			t.Fatal(err)
		}
		for i, v := range b {
			if v != 0 {
				t.Fatalf("shape=%s byte%d leaked%x", shape, i, v)
			}
		}
		if err := j.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSparseJobMemoryReuseGrown(t *testing.T) {
	saved := sparseJobMemoryReuseEnabled
	sparseJobMemoryReuseEnabled = true
	defer func() { sparseJobMemoryReuseEnabled = saved }()
	j, err := AcquireJobMemoryGrowable(65536, 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := j.GrowShared(31, 0); !ok {
		t.Fatal("grow failed")
	}
	j.CurrentBytes()[65536+8] = 0xa5
	j.CurrentBytes()[(2<<20)-1] = 0xa5
	base := j.LinMemBase()
	if err := ReleaseJobMemory(j); err != nil {
		t.Fatal(err)
	}
	next, err := AcquireJobMemoryGrowable(65536, 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseJobMemory(next)
	if next.LinMemBase() != base {
		t.Fatal("mapping not reused")
	}
	if _, ok := next.GrowShared(31, 0); !ok {
		t.Fatal("regrow failed")
	}
	for i, v := range next.CurrentBytes() {
		if v != 0 {
			t.Fatalf("grown byte%d leaked%x", i, v)
		}
	}
}
