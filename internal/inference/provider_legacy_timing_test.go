package inference

import (
	"testing"
	"time"
)

func TestProviderSchedulerAggregatesCompletedLegacyTiming(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	first := ProviderTiming{Load: 2 * time.Millisecond, Queue: time.Millisecond, Generation: 3 * time.Millisecond}
	second := ProviderTiming{Queue: 4 * time.Millisecond, Generation: 5 * time.Millisecond, Cancel: time.Millisecond}
	scheduler.RecordLegacyTiming(first)
	scheduler.RecordLegacyTiming(second)
	got := scheduler.LegacyTimingSnapshot()
	if got.Samples != 2 || got.Load != int64(first.Load) || got.Queue != int64(first.Queue+second.Queue) || got.Generation != int64(first.Generation+second.Generation) || got.Cancel != int64(second.Cancel) {
		t.Fatalf("legacy timing aggregation is incomplete: %+v", got)
	}
}
