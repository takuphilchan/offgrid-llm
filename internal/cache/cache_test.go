package cache

import (
	"sync"
	"testing"
	"time"
)

func TestCleanupLifecycle(t *testing.T) {
	c := NewResponseCache(10, time.Minute)
	c.StopCleanupRoutine() // Safe before start and after stop.
	for range 3 {
		c.StartCleanupRoutine(time.Hour)
		done := c.cleanupDone
		c.StartCleanupRoutine(time.Hour)
		if c.cleanupDone != done {
			t.Fatal("starting cleanup twice created a second worker")
		}
		c.StopCleanupRoutine()
		select {
		case <-done:
		default:
			t.Fatal("stop returned before the worker exited")
		}
		c.StopCleanupRoutine()
	}
}

func TestConcurrentCacheLifecycle(t *testing.T) {
	c := NewResponseCache(10, time.Minute)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				c.StartCleanupRoutine(time.Hour)
				c.Enable()
				c.Set("model", "prompt", "answer", nil)
				c.Get("model", "prompt", nil)
				c.Stats()
				c.Disable()
				c.StopCleanupRoutine()
			}
		}()
	}
	workers.Wait()
	c.StopCleanupRoutine()
	c.Disable()
	c.Set("model", "disabled", "answer", nil)
	c.Enable()
	if _, found := c.Get("model", "disabled", nil); found {
		t.Fatal("disabled cache accepted a write")
	}
}
