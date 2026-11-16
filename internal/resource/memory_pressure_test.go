package resource

import "testing"

func TestHostMemoryRespectsCommitWithoutRelabelingRAM(t *testing.T) {
	for _, tc := range []struct {
		physical, commit, want int64
		measured               bool
	}{
		{8000, 0, 8000, false}, {8000, 0, 0, true}, {8000, 1200, 1200, true},
		{1200, 8000, 1200, true}, {8000, -1, 0, true}, {-1, 8000, 0, true},
	} {
		r := &SystemResources{AvailableRAM: tc.physical, AvailableCommitMB: tc.commit, CommitMeasured: tc.measured}
		if got := r.AvailableHostMemoryMB(); got != tc.want {
			t.Fatalf("%+v: got %d", tc, got)
		}
		if r.AvailableRAM != tc.physical {
			t.Fatal("physical RAM overwritten")
		}
	}
}

func TestCurrentMemorySample(t *testing.T) {
	r, err := DetectMemoryResources()
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalRAM <= 0 || r.AvailableRAM < 0 || r.AvailableRAM > r.TotalRAM {
		t.Fatalf("bad sample: %+v", r)
	}
	if r.OS == "windows" && (!r.CommitMeasured || r.AvailableCommitMB < 0) {
		t.Fatalf("commit unmeasured: %+v", r)
	}
	t.Logf("physical available %d MiB of %d MiB; commit measured=%t, available=%d MiB", r.AvailableRAM, r.TotalRAM, r.CommitMeasured, r.AvailableCommitMB)
}
