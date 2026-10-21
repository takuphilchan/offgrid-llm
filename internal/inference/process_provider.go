package inference

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// ProcessProvider supervises a trusted local worker speaking the newline JSON
// protocol. The command path is supplied by service configuration, never by a
// model manifest or an agent response.
type ProcessProvider struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Scanner
	descriptor ProviderDescriptor
	resources  ProviderResources
	mu         sync.Mutex
	readSlot   chan struct{}
	closedCh   chan struct{}
	writeMu    sync.Mutex
	seen       map[string]struct{}
	release    func()
	terminate  func(*exec.Cmd) error
	closed     bool
	closeDone  chan struct{}
	closeErr   error
}

// A failed startup is not proof that its child was stopped. Callers that hold
// model-package leases must retain them when this sentinel accompanies the
// startup error.
var errProviderStopUncertain = errors.New("provider worker stop uncertain")

func (p *ProcessProvider) rejectStartup(cause error) error {
	if closeErr := p.Close(); closeErr != nil {
		return errors.Join(cause, fmt.Errorf("%w: %v", errProviderStopUncertain, closeErr))
	}
	return cause
}

func StartProcessProvider(ctx context.Context, command string, args []string, descriptor ProviderDescriptor, env []string) (*ProcessProvider, error) {
	if command == "" {
		return nil, errors.New("provider worker command is required")
	}
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The startup context governs admission only. Binding the worker process
	// lifetime to the first generation context would kill a resident provider
	// after that request and make reuse/recovery unreliable.
	cmd := exec.Command(command, args...)
	configureBackgroundProcess(cmd)
	cmd.Env = append([]string{}, env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open provider stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("open provider stdout: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := startOwnedProcess(cmd); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("start provider worker: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	p := &ProcessProvider{cmd: cmd, stdin: stdin, stdout: scanner, descriptor: descriptor, readSlot: make(chan struct{}, 1), closedCh: make(chan struct{}), seen: make(map[string]struct{}), terminate: terminateProviderProcess}
	ready := make(chan error, 1)
	go func() {
		line, err := p.readLine()
		if err != nil {
			ready <- fmt.Errorf("provider handshake: %w", err)
			return
		}
		if err := jsonwire.Validate(line); err != nil {
			ready <- fmt.Errorf("ambiguous provider handshake: %w", err)
			return
		}
		var event ProviderEvent
		if err := json.Unmarshal(line, &event); err != nil {
			ready <- fmt.Errorf("invalid provider handshake: %w", err)
			return
		}
		if event.Type == ProviderEventError && event.Error != nil {
			ready <- fmt.Errorf("provider startup %s: %s", event.Error.Code, event.Error.Message)
			return
		}
		if event.ProtocolRevision != ProviderProtocolRevision || event.Type != ProviderEventReady || event.RequestID != "" || event.Sequence != 0 || event.Descriptor == nil || !reflect.DeepEqual(*event.Descriptor, descriptor) || event.Resources == nil {
			ready <- errors.New("provider handshake identity mismatch")
			return
		}
		if err := event.Resources.Validate(descriptor); err != nil {
			ready <- fmt.Errorf("provider handshake resource mismatch: %w", err)
			return
		}
		p.mu.Lock()
		p.resources = *event.Resources
		p.mu.Unlock()
		ready <- nil
	}()
	select {
	case err := <-ready:
		if err != nil {
			return nil, p.rejectStartup(err)
		}
		if err := ctx.Err(); err != nil {
			return nil, p.rejectStartup(err)
		}
	case <-ctx.Done():
		return nil, p.rejectStartup(ctx.Err())
	case <-time.After(2 * time.Minute):
		return nil, p.rejectStartup(errors.New("provider handshake timed out"))
	}
	return p, nil
}

// NewMLXProcessProvider and NewTransformersProcessProvider are explicit
// service-owned entry points. The executable and model path come from trusted
// runtime configuration; neither can be supplied by a model response.
func NewMLXProcessProvider(ctx context.Context, pythonCommand, workerScript, modelPath string, descriptor ProviderDescriptor, dependencies map[string]string, env []string) (*ProcessProvider, error) {
	if descriptor.Provider != ProviderMLX || descriptor.Format != FormatMLXSafetensors {
		return nil, errors.New("MLX provider requires an MLX safetensors descriptor")
	}
	return startPythonProcessProvider(ctx, pythonCommand, workerScript, modelPath, descriptor, dependencies, env)
}

func NewTransformersProcessProvider(ctx context.Context, pythonCommand, workerScript, modelPath string, descriptor ProviderDescriptor, dependencies map[string]string, env []string) (*ProcessProvider, error) {
	if descriptor.Provider != ProviderTransformers || descriptor.Format != FormatSafetensors {
		return nil, errors.New("Transformers provider requires a safetensors descriptor")
	}
	return startPythonProcessProvider(ctx, pythonCommand, workerScript, modelPath, descriptor, dependencies, env)
}

func startPythonProcessProvider(ctx context.Context, pythonCommand, workerScript, modelPath string, descriptor ProviderDescriptor, dependencies map[string]string, env []string) (*ProcessProvider, error) {
	if descriptor.Provider == ProviderMLX && descriptor.Backend != "metal" {
		return nil, errors.New("MLX text worker requires Metal backend")
	}
	if descriptor.Provider == ProviderTransformers && descriptor.Backend != "cpu" && descriptor.Backend != "cuda" && descriptor.Backend != "mps" {
		return nil, errors.New("Transformers text worker requires CPU, CUDA, or MPS backend")
	}
	for _, capability := range descriptor.Capabilities {
		if capability == CapabilityToolCalls || capability == CapabilityVision || capability == CapabilityAudio {
			return nil, errors.New("native provider multimodal and tool capabilities are not qualified")
		}
	}
	if !filepath.IsAbs(pythonCommand) || !filepath.IsAbs(workerScript) || !filepath.IsAbs(modelPath) {
		return nil, errors.New("provider executable, script, and model paths must be absolute")
	}
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	if _, err := (TrustedNativeWorker{Provider: descriptor.Provider, DependencyVersions: dependencies}).dependencyIdentity(); err != nil {
		return nil, err
	}
	workerEnv, err := nativeWorkerEnvironment(env)
	if err != nil {
		return nil, err
	}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		return nil, err
	}
	dependencyIdentity, err := json.Marshal(dependencies)
	if err != nil {
		return nil, err
	}
	// Isolated mode ignores PYTHONPATH, user site packages, and the current
	// directory. A reviewed worker must load only from its pinned interpreter's
	// environment, not from an actor-controlled launch directory. Disable
	// bytecode writes explicitly because isolated mode ignores PYTHON* env vars.
	args := isolatedPythonWorkerArgs(workerScript, modelPath, descriptor, identity, dependencyIdentity)
	return StartProcessProvider(ctx, pythonCommand, args, descriptor, workerEnv)
}

func isolatedPythonWorkerArgs(workerScript, modelPath string, descriptor ProviderDescriptor, identity, dependencies []byte) []string {
	return []string{"-I", "-B", workerScript, "--provider", string(descriptor.Provider), "--model", modelPath, "--backend", descriptor.Backend, "--descriptor", base64.RawURLEncoding.EncodeToString(identity), "--dependencies", base64.RawURLEncoding.EncodeToString(dependencies)}
}

func nativeWorkerEnvironment(extra []string) ([]string, error) {
	base := []string{"HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "HF_HUB_DISABLE_IMPLICIT_TOKEN=1", "PYTHONNOUSERSITE=1", "PYTHONDONTWRITEBYTECODE=1"}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SystemRoot", "WINDIR"} {
			if value := os.Getenv(key); value != "" {
				base = append(base, key+"="+value)
			}
		}
	}
	allowed := map[string]bool{"CUDA_VISIBLE_DEVICES": true, "HIP_VISIBLE_DEVICES": true, "ROCR_VISIBLE_DEVICES": true, "OMP_NUM_THREADS": true}
	for _, setting := range extra {
		key, value, ok := strings.Cut(setting, "=")
		if !ok || !allowed[key] || value == "" || strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("unsupported provider worker environment key %q", key)
		}
		base = append(base, setting)
	}
	return base, nil
}

func (p *ProcessProvider) Descriptor(context.Context) (ProviderDescriptor, error) {
	return p.descriptor, nil
}

func (p *ProcessProvider) Resources(context.Context) (ProviderResources, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ProviderResources{}, errors.New("provider worker is closed")
	}
	return p.resources, nil
}

func (p *ProcessProvider) readLine() ([]byte, error) {
	if !p.stdout.Scan() {
		if err := p.stdout.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return p.stdout.Bytes(), nil
}

func (p *ProcessProvider) readLineWithContext(ctx context.Context) ([]byte, error) {
	type result struct {
		line []byte
		err  error
	}
	ready := make(chan result, 1)
	go func() {
		line, err := p.readLine()
		ready <- result{line, err}
	}()
	select {
	case value := <-ready:
		return value.line, value.err
	case <-ctx.Done():
		// A timed-out worker has an unknown output boundary. Terminating it
		// also unblocks Scan before another request can be admitted.
		_ = p.cmd.Process.Kill()
		<-ready
		return nil, ctx.Err()
	}
}

// A worker has one stdout protocol stream. Waiting for that stream must be
// cancellable: a plain mutex would leave a queued request stuck behind a long
// generation even after Stop was acknowledged by the scheduler.
func (p *ProcessProvider) acquireRead(ctx context.Context) (func(), error) {
	select {
	case p.readSlot <- struct{}{}:
		select {
		case <-p.closedCh:
			<-p.readSlot
			return nil, errors.New("provider worker is closed")
		default:
		}
		return func() { <-p.readSlot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.closedCh:
		return nil, errors.New("provider worker is closed")
	}
}

func (p *ProcessProvider) write(ctx context.Context, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// An owned worker may stop reading while its pipe fills. A context check
	// before Write alone cannot keep Stop responsive: Write itself can block.
	// Kill only this worker on cancellation, which also invalidates the stream
	// boundary so the request cannot be retried on this process.
	written := make(chan error, 1)
	go func() {
		p.writeMu.Lock()
		defer p.writeMu.Unlock()
		_, writeErr := p.stdin.Write(append(data, '\n'))
		written <- writeErr
	}()
	select {
	case err := <-written:
		return err
	case <-ctx.Done():
		_ = p.cmd.Process.Kill()
		_ = p.stdin.Close()
		return ctx.Err()
	}
}

func (p *ProcessProvider) Generate(ctx context.Context, req ProviderRequest, callback func(ProviderEvent) error) error {
	if err := req.Validate(); err != nil {
		return err
	}
	if req.ModelDigest != p.descriptor.ModelDigest {
		return errors.New("provider request model digest mismatch")
	}
	if p.descriptor.ContextLength > 0 && req.ContextLength > p.descriptor.ContextLength {
		return errors.New("provider request exceeds the loaded model context")
	}
	if callback == nil {
		return errors.New("provider callback is required")
	}
	if len(req.Tools) != 0 {
		return errors.New("native provider tool decisions are not qualified")
	}
	releaseRead, err := p.acquireRead(ctx)
	if err != nil {
		return err
	}
	defer releaseRead()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return errors.New("provider worker is closed")
	}
	if _, used := p.seen[req.RequestID]; used || len(p.seen) >= 10000 {
		p.mu.Unlock()
		return errors.New("provider request identity was already used or exhausted")
	}
	p.seen[req.RequestID] = struct{}{}
	p.mu.Unlock()
	if err := p.write(ctx, struct {
		Operation string          `json:"operation"`
		Request   ProviderRequest `json:"request"`
	}{"generate", req}); err != nil {
		_ = p.cmd.Process.Kill()
		return err
	}
	clean := false
	defer func() {
		// A malformed, truncated, or aborted stream leaves the worker's next
		// response boundary unknown. Do not feed it another request.
		if !clean {
			_ = p.cmd.Process.Kill()
		}
	}()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-finished:
			return
		case <-ctx.Done():
			cancelCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = p.Cancel(cancelCtx, req.CancellationID)
			cancel()
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				_ = p.cmd.Process.Kill()
			}
		}
	}()
	var sequence uint64
	for {
		line, err := p.readLine()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("provider worker stream: %w", err)
		}
		if err := jsonwire.Validate(line); err != nil {
			return fmt.Errorf("ambiguous provider worker event: %w", err)
		}
		var event ProviderEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return fmt.Errorf("provider worker emitted invalid event: %w", err)
		}
		if event.RequestID != req.RequestID {
			return fmt.Errorf("provider worker event request mismatch: %q", event.RequestID)
		}
		if event.ProtocolRevision != p.descriptor.ProtocolRevision || event.Sequence <= sequence {
			return errors.New("provider worker event protocol or sequence mismatch")
		}
		sequence = event.Sequence
		if err := validateProviderEvent(event); err != nil {
			return err
		}
		if event.Usage != nil && req.ContextLength > 0 && event.Usage.TotalTokens > req.ContextLength {
			return errors.New("provider usage exceeds the admitted request context")
		}
		if event.Resources != nil {
			if event.Type != ProviderEventDone {
				return errors.New("provider resource report is only allowed at completion")
			}
			if err := event.Resources.Validate(p.descriptor); err != nil {
				return fmt.Errorf("provider completion resource mismatch: %w", err)
			}
			p.mu.Lock()
			p.resources = *event.Resources
			p.mu.Unlock()
		}
		if event.Type == ProviderEventTool || event.Type == ProviderEventDone && event.FinishReason == "tool_calls" {
			return errors.New("native provider tool decisions are not qualified")
		}
		if event.Type == ProviderEventError {
			// A complete, validated error event is a terminal protocol boundary.
			// Keep the resident worker for the next explicit request; malformed or
			// truncated output still invalidates and terminates it below.
			clean = true
			return fmt.Errorf("provider %s: %s", event.Error.Code, event.Error.Message)
		}
		if event.Type == ProviderEventDone && ctx.Err() != nil {
			return ctx.Err()
		}
		if err := callback(event); err != nil {
			return err
		}
		if event.Type == ProviderEventDone {
			clean = true
			if event.FinishReason == "cancelled" {
				return context.Canceled
			}
			return nil
		}
	}
}

func validateProviderEvent(event ProviderEvent) error {
	if event.Descriptor != nil ||
		(event.Type != ProviderEventDelta && event.Text != "") ||
		(event.Type != ProviderEventTool && len(event.Tool) != 0) ||
		(event.Type != ProviderEventDone && event.FinishReason != "") ||
		(event.Type != ProviderEventError && event.Error != nil) ||
		(event.Type != ProviderEventUsage && event.Type != ProviderEventDone && event.Usage != nil) ||
		(event.Type != ProviderEventDone && event.Resources != nil) {
		return errors.New("provider generation event contains fields from another event type")
	}
	if event.Usage != nil {
		usage := event.Usage
		if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 || usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
			return errors.New("provider token usage is inconsistent")
		}
	}
	switch event.Type {
	case ProviderEventDelta:
		if event.Text == "" {
			return errors.New("provider delta cannot be empty")
		}
	case ProviderEventTool:
		if len(event.Tool) == 0 || !json.Valid(event.Tool) {
			return errors.New("provider tool decision must be valid JSON")
		}
	case ProviderEventUsage:
		if event.Usage == nil {
			return errors.New("provider usage event has no token counts")
		}
	case ProviderEventDone:
		if event.FinishReason != "stop" && event.FinishReason != "length" && event.FinishReason != "cancelled" && event.FinishReason != "tool_calls" {
			return errors.New("provider terminal event has no supported finish reason")
		}
	case ProviderEventError:
		if event.Error == nil || event.Error.Code == "" || event.Error.Message == "" {
			return errors.New("provider error event is incomplete")
		}
	default:
		return fmt.Errorf("provider emitted unsupported event type %q", event.Type)
	}
	return nil
}

func (p *ProcessProvider) Cancel(ctx context.Context, cancellationID string) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil
	}
	return p.write(ctx, struct {
		Operation string `json:"operation"`
		ID        string `json:"cancellation_id"`
	}{"cancel", cancellationID})
}

func (p *ProcessProvider) Health(ctx context.Context) error {
	releaseRead, err := p.acquireRead(ctx)
	if err != nil {
		return err
	}
	defer releaseRead()
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return errors.New("provider worker is closed")
	}
	// Once a health request is attempted, an invalid or missing reply leaves
	// stdout alignment and worker state uncertain. Retire the owned process so
	// no later caller can mistake a stale reply for a fresh generation event.
	healthy := false
	defer func() {
		if !healthy {
			_ = p.cmd.Process.Kill()
		}
	}()
	if err := p.write(ctx, map[string]string{"operation": "health"}); err != nil {
		return err
	}
	line, err := p.readLineWithContext(ctx)
	if err != nil {
		return fmt.Errorf("provider health stream: %w", err)
	}
	if err := jsonwire.Validate(line); err != nil {
		return fmt.Errorf("ambiguous provider health event: %w", err)
	}
	var event ProviderEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return fmt.Errorf("provider health emitted invalid event: %w", err)
	}
	if event.Type != ProviderEventReady || event.ProtocolRevision != p.descriptor.ProtocolRevision || event.RequestID != "" || event.Sequence != 0 || event.Descriptor != nil || event.Resources == nil {
		return fmt.Errorf("provider health returned %q", event.Type)
	}
	if err := event.Resources.Validate(p.descriptor); err != nil {
		return fmt.Errorf("provider health resource mismatch: %w", err)
	}
	p.mu.Lock()
	p.resources = *event.Resources
	p.mu.Unlock()
	healthy = true
	return nil
}

func (p *ProcessProvider) Close() error {
	p.mu.Lock()
	if p.closed {
		done := p.closeDone
		p.mu.Unlock()
		<-done
		p.mu.Lock()
		err := p.closeErr
		p.mu.Unlock()
		return err
	}
	p.closed = true
	p.closeDone = make(chan struct{})
	close(p.closedCh)
	p.mu.Unlock()
	_ = p.stdin.Close()
	closeErr := p.terminate(p.cmd)
	if closeErr == nil && p.release != nil {
		p.release()
	}
	p.mu.Lock()
	p.closeErr = closeErr
	close(p.closeDone)
	p.mu.Unlock()
	return closeErr
}

func terminateProviderProcess(cmd *exec.Cmd) error {
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) && !ownedProcessExitedAfterKill(cmd.Process) {
		return err
	}
	// ExitError is expected after terminating an owned, healthy worker. A
	// process that cannot be reaped is not proven stopped, so its package
	// lease must remain held rather than permitting replacement underneath it.
	waitErr := cmd.Wait()
	var exited *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exited) && !errors.Is(waitErr, os.ErrProcessDone) {
		return waitErr
	}
	return nil
}
