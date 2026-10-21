package inference

import (
	"context"
	"log"
	"time"
)

// monitorProcesses periodically checks llama-server children and restarts
// crashed instances. StopMonitor cancels any in-flight restart and joins this
// goroutine before provider shutdown retires the remaining processes.
func (mc *ModelCache) monitorProcesses() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer close(mc.monitorDone)
	for {
		select {
		case <-mc.stopMonitor:
			return
		case <-ticker.C:
			select {
			case <-mc.stopMonitor:
				return
			default:
			}
			mc.checkAndRestartCrashed(mc.monitorCtx)
		}
	}
}

func (mc *ModelCache) checkAndRestartCrashed(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	mc.mu.Lock()
	if !mc.autoRestart {
		mc.mu.Unlock()
		return
	}
	var crashed []struct {
		modelID       string
		modelPath     string
		projectorPath string
		instance      *ModelInstance
	}
	for modelID, instance := range mc.instances {
		if _, loading := mc.pendingLoads[modelID]; loading {
			continue
		}
		if instance.Cmd == nil || instance.Cmd.Process == nil {
			continue
		}
		if !mc.instanceHealthy(instance) {
			crashed = append(crashed, struct {
				modelID       string
				modelPath     string
				projectorPath string
				instance      *ModelInstance
			}{modelID, instance.ModelPath, instance.ProjectorPath, instance})
		}
	}
	admit := mc.restartAdmission
	mc.mu.Unlock()
	if len(crashed) == 0 {
		return
	}
	if admit != nil {
		release, err := admit(ctx)
		if err != nil {
			log.Printf("Cannot admit crashed llama-server restart: %v", err)
			return
		}
		if release == nil {
			log.Printf("Cannot admit crashed llama-server restart: missing release lease")
			return
		}
		defer release()
	}
	for _, c := range crashed {
		if ctx.Err() != nil {
			return
		}
		mc.mu.Lock()
		instance, exists := mc.instances[c.modelID]
		if !mc.autoRestart || !exists || instance != c.instance || mc.pendingLoads[c.modelID] != nil || mc.instanceHealthy(instance) {
			mc.mu.Unlock()
			continue
		}
		log.Printf("Detected crashed llama-server for model %s, will restart...", c.modelID)
		mc.cleanupInstance(c.modelID)
		mc.mu.Unlock()
		log.Printf("Restarting model %s...", c.modelID)
		_, err := mc.GetOrLoadContext(ctx, c.modelID, c.modelPath, c.projectorPath)
		if err != nil {
			log.Printf("Failed to restart model %s: %v", c.modelID, err)
		} else {
			log.Printf("Successfully restarted model %s", c.modelID)
		}
	}
}

// SetRestartAdmission attaches the shared scheduler's exclusive llama mutation
// lease. Standalone cache tests can continue using the legacy direct path.
func (mc *ModelCache) SetRestartAdmission(admit func(context.Context) (func(), error)) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.restartAdmission = admit
}

// SetAutoRestart enables or disables automatic restart of crashed processes.
func (mc *ModelCache) SetAutoRestart(enable bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.autoRestart = enable
}

// StopMonitor cancels restarts and waits for the process monitor to exit.
func (mc *ModelCache) StopMonitor() {
	if mc.stopMonitor == nil {
		return
	}
	mc.monitorStop.Do(func() {
		if mc.monitorCancel != nil {
			mc.monitorCancel()
		}
		close(mc.stopMonitor)
	})
	if mc.monitorDone != nil {
		<-mc.monitorDone
	}
}
