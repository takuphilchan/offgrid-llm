package inference

import "context"

// SetLoadAdmission installs the service's memory check before any model loads.
// It runs after eviction, before starting a new process, under the cache lock;
// it must not call back into the cache. Resident reuse does not re-admit memory.
func (mc *ModelCache) SetLoadAdmission(check func(context.Context, string, int) error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.loadAdmission = check
}
