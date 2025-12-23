// Package watchdog provides process monitoring and automatic recovery
// for llama-server and other critical services in offline/edge deployments.
package watchdog

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ProcessState represents the current state of a monitored process
type ProcessState string

const (
	StateRunning   ProcessState = "running"
	StateStopped   ProcessState = "stopped"
	StateStarting  ProcessState = "starting"
	StateFailed    ProcessState = "failed"
	StateUnhealthy ProcessState = "unhealthy"
)

// ProcessConfig defines how to monitor and manage a process
type ProcessConfig struct {
	Name            string        // Human-readable name
	HealthEndpoint  string        // HTTP health check URL (e.g., "http://localhost:42382/health")
	HealthTimeout   time.Duration // Timeout for health checks
	CheckInterval   time.Duration // How often to check health
	RestartDelay    time.Duration // Delay before restarting after failure
	MaxRestarts     int           // Max restarts within RestartWindow (0 = unlimited)
	RestartWindow   time.Duration // Time window for counting restarts
	StartCommand    string        // Command to start the process
	StartArgs       []string      // Arguments for start command
	GracefulTimeout time.Duration // Timeout for graceful shutdown
}

// ProcessStatus contains current status information
type ProcessStatus struct {
	Name         string       `json:"name"`
	State        ProcessState `json:"state"`
	PID          int          `json:"pid,omitempty"`
	Uptime       string       `json:"uptime,omitempty"`
	LastCheck    time.Time    `json:"last_check"`
	LastRestart  time.Time    `json:"last_restart,omitempty"`
	RestartCount int          `json:"restart_count"`
	LastError    string       `json:"last_error,omitempty"`
}

// Watchdog monitors and manages critical processes
type Watchdog struct {
	mu            sync.RWMutex
	processes     map[string]*monitoredProcess
	httpClient    *http.Client
	stopChan      chan struct{}
	running       bool
	onStateChange func(name string, oldState, newState ProcessState)
	logger        *log.Logger
}

type monitoredProcess struct {
	config       ProcessConfig
	cmd          *exec.Cmd
	state        ProcessState
	startTime    time.Time
	restartTimes []time.Time
	lastError    error
	stopChan     chan struct{}
}

// New creates a new Watchdog instance
func New() *Watchdog {
	return &Watchdog{
		processes: make(map[string]*monitoredProcess),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		stopChan: make(chan struct{}),
		logger:   log.New(os.Stdout, "[WATCHDOG] ", log.LstdFlags),
	}
}

// SetLogger sets a custom logger
func (w *Watchdog) SetLogger(logger *log.Logger) {
	w.logger = logger
}

// OnStateChange sets a callback for state changes
func (w *Watchdog) OnStateChange(callback func(name string, oldState, newState ProcessState)) {
	w.onStateChange = callback
}

// Register adds a process to be monitored
func (w *Watchdog) Register(config ProcessConfig) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if config.Name == "" {
		return fmt.Errorf("process name is required")
	}

	if config.HealthTimeout == 0 {
		config.HealthTimeout = 5 * time.Second
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 10 * time.Second
	}
	if config.RestartDelay == 0 {
		config.RestartDelay = 5 * time.Second
	}
	if config.RestartWindow == 0 {
		config.RestartWindow = 5 * time.Minute
	}
	if config.GracefulTimeout == 0 {
		config.GracefulTimeout = 3 * time.Minute // Increased for low-end machines
	}

	w.processes[config.Name] = &monitoredProcess{
		config:   config,
		state:    StateStopped,
		stopChan: make(chan struct{}),
	}

	return nil
}

// Start begins monitoring all registered processes
func (w *Watchdog) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watchdog already running")
	}
	w.running = true
	w.mu.Unlock()

	w.logger.Println("Starting watchdog service...")

	for name, proc := range w.processes {
		go w.monitorProcess(ctx, name, proc)
	}

	return nil
}

// Stop gracefully stops the watchdog and all monitored processes
func (w *Watchdog) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	w.logger.Println("Stopping watchdog service...")
	close(w.stopChan)

	// Stop all monitored processes
	for name, proc := range w.processes {
		w.stopProcess(name, proc)
	}
}

// GetStatus returns the status of all monitored processes
func (w *Watchdog) GetStatus() map[string]ProcessStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	status := make(map[string]ProcessStatus)
	for name, proc := range w.processes {
		s := ProcessStatus{
			Name:         name,
			State:        proc.state,
			LastCheck:    time.Now(),
			RestartCount: len(proc.restartTimes),
		}

		if proc.cmd != nil && proc.cmd.Process != nil {
			s.PID = proc.cmd.Process.Pid
		}

		if !proc.startTime.IsZero() {
			s.Uptime = time.Since(proc.startTime).Round(time.Second).String()
		}

		if len(proc.restartTimes) > 0 {
			s.LastRestart = proc.restartTimes[len(proc.restartTimes)-1]
		}

		if proc.lastError != nil {
			s.LastError = proc.lastError.Error()
		}

		status[name] = s
	}

	return status
}

// GetProcessStatus returns the status of a specific process
func (w *Watchdog) GetProcessStatus(name string) (ProcessStatus, bool) {
	statuses := w.GetStatus()
	status, exists := statuses[name]
	return status, exists
}

// RestartProcess manually triggers a restart of a process
func (w *Watchdog) RestartProcess(name string) error {
	w.mu.RLock()
	proc, exists := w.processes[name]
	w.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	w.logger.Printf("Manual restart requested for %s", name)
	w.stopProcess(name, proc)
	return w.startProcess(name, proc)
}

func (w *Watchdog) monitorProcess(ctx context.Context, name string, proc *monitoredProcess) {
	ticker := time.NewTicker(proc.config.CheckInterval)
	defer ticker.Stop()

	// Initial start
	if err := w.startProcess(name, proc); err != nil {
		w.logger.Printf("Failed to start %s: %v", name, err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-proc.stopChan:
			return
		case <-ticker.C:
			w.checkAndRecover(name, proc)
		}
	}
}

func (w *Watchdog) checkAndRecover(name string, proc *monitoredProcess) {
	healthy := w.checkHealth(proc)

	w.mu.Lock()
	oldState := proc.state

	if healthy {
		if proc.state != StateRunning {
			w.setState(name, proc, StateRunning)
		}
		w.mu.Unlock()
		return
	}

	// Not healthy
	if proc.state == StateRunning {
		w.setState(name, proc, StateUnhealthy)
	}
	w.mu.Unlock()

	// Check if we've exceeded max restarts
	if proc.config.MaxRestarts > 0 {
		recentRestarts := w.countRecentRestarts(proc)
		if recentRestarts >= proc.config.MaxRestarts {
			w.mu.Lock()
			if proc.state != StateFailed {
				w.setState(name, proc, StateFailed)
				w.logger.Printf("Process %s exceeded max restarts (%d in %v), giving up",
					name, proc.config.MaxRestarts, proc.config.RestartWindow)
			}
			w.mu.Unlock()
			return
		}
	}

	// Attempt restart
	w.logger.Printf("Process %s unhealthy (was %s), attempting restart...", name, oldState)
	time.Sleep(proc.config.RestartDelay)

	if err := w.startProcess(name, proc); err != nil {
		w.logger.Printf("Failed to restart %s: %v", name, err)
		proc.lastError = err
	}
}

func (w *Watchdog) checkHealth(proc *monitoredProcess) bool {
	// First check if process is still running
	if proc.cmd != nil && proc.cmd.Process != nil {
		if err := proc.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return false
		}
	}

	// Then check HTTP health endpoint if configured
	if proc.config.HealthEndpoint != "" {
		client := &http.Client{Timeout: proc.config.HealthTimeout}
		resp, err := client.Get(proc.config.HealthEndpoint)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}

	// No health endpoint, just check if process exists
	return proc.cmd != nil && proc.cmd.Process != nil
}

func (w *Watchdog) startProcess(name string, proc *monitoredProcess) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if proc.config.StartCommand == "" {
		// No start command configured, just monitor existing process
		return nil
	}

	w.setState(name, proc, StateStarting)

	cmd := exec.Command(proc.config.StartCommand, proc.config.StartArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		w.setState(name, proc, StateFailed)
		proc.lastError = err
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	proc.cmd = cmd
	proc.startTime = time.Now()
	proc.restartTimes = append(proc.restartTimes, time.Now())

	// Wait a moment for process to initialize
	time.Sleep(1 * time.Second)

	w.logger.Printf("Started %s (PID: %d)", name, cmd.Process.Pid)
	return nil
}

func (w *Watchdog) stopProcess(name string, proc *monitoredProcess) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if proc.cmd == nil || proc.cmd.Process == nil {
		return
	}

	w.logger.Printf("Stopping %s (PID: %d)...", name, proc.cmd.Process.Pid)

	// Try graceful shutdown first
	proc.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- proc.cmd.Wait()
	}()

	select {
	case <-done:
		w.logger.Printf("Process %s stopped gracefully", name)
	case <-time.After(proc.config.GracefulTimeout):
		w.logger.Printf("Process %s didn't stop gracefully, killing...", name)
		proc.cmd.Process.Kill()
	}

	proc.cmd = nil
	w.setState(name, proc, StateStopped)
}

func (w *Watchdog) setState(name string, proc *monitoredProcess, newState ProcessState) {
	oldState := proc.state
	proc.state = newState

	if w.onStateChange != nil && oldState != newState {
		go w.onStateChange(name, oldState, newState)
	}
}

func (w *Watchdog) countRecentRestarts(proc *monitoredProcess) int {
	cutoff := time.Now().Add(-proc.config.RestartWindow)
	count := 0
	for _, t := range proc.restartTimes {
		if t.After(cutoff) {
			count++
		}
	}
	return count
}

// DefaultLlamaServerConfig returns a default configuration for monitoring llama-server
func DefaultLlamaServerConfig(port int, modelPath string) ProcessConfig {
	return ProcessConfig{
		Name:            "llama-server",
		HealthEndpoint:  fmt.Sprintf("http://localhost:%d/health", port),
		HealthTimeout:   30 * time.Second, // Increased for low-end machines
		CheckInterval:   30 * time.Second, // Increased for low-end machines
		RestartDelay:    10 * time.Second, // Increased for low-end machines
		MaxRestarts:     5,
		RestartWindow:   10 * time.Minute, // Increased for low-end machines
		GracefulTimeout: 3 * time.Minute,  // Increased for low-end machines
		StartCommand:    "llama-server",
		StartArgs: []string{
			"-m", modelPath,
			"--port", fmt.Sprintf("%d", port),
			"--host", "127.0.0.1",
		},
	}
}
