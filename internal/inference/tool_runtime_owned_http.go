//go:build !llama

package inference

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
)

// ObserveToolRuntime addresses the selected owned child's endpoint, never the
// mutable shared engine URL. It neither starts nor reconnects a model. Readiness
// and a declared template alone do not qualify a runtime for agent execution.
func (p *LlamaCPPProcess) ObserveToolRuntime(ctx context.Context, model string, window int) (*ToolRuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil || model == "" || window <= 0 {
		return nil, errors.New("owned tool runtime observation is unavailable")
	}
	cache, ok := p.cache.(*ModelCache)
	if !ok || cache == nil {
		return nil, errors.New("owned tool runtime cache is unavailable")
	}
	cache.mu.RLock()
	instance := cache.instances[model]
	if instance == nil || instance.Generation == 0 || instance.ModelID != model || instance.Port < 1 || instance.Port > 65535 || instance.Cmd == nil || instance.Cmd.Process == nil || instance.ContextSize < window {
		cache.mu.RUnlock()
		return nil, errors.New("selected tool runtime is not resident at the requested context")
	}
	generation, port, process := instance.Generation, instance.Port, instance.Cmd.Process
	args := append([]string(nil), instance.Cmd.Args...)
	var environment []string
	if instance.Cmd.Env != nil {
		environment = append([]string{}, instance.Cmd.Env...)
	}
	modelPath, projectorPath, executablePath := instance.ModelPath, instance.ProjectorPath, instance.Cmd.Path
	cache.mu.RUnlock()
	if !ownedProcessAlive(process) {
		return nil, errors.New("selected tool runtime process is unavailable")
	}
	// Do not hold the cache mutex across HTTP. Stop/restart must not wait for
	// an unresponsive metadata endpoint. The generation fence below detects it.
	endpoint := NewLlamaHTTPEngine(fmt.Sprintf("http://127.0.0.1:%d", port))
	defer endpoint.httpClient.CloseIdleConnections()
	endpoint.httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	info, err := endpoint.ToolRuntimeInfo(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cache.mu.RLock()
	current := cache.instances[model]
	unchanged := current == instance && current.Generation == generation && current.Port == port && current.Cmd != nil && current.Cmd.Process == process
	cache.mu.RUnlock()
	if !unchanged || !ownedProcessAlive(process) {
		return nil, errors.New("tool runtime process changed during observation")
	}
	if info == nil || info.Context < window || info.Build == "" || info.TemplateSHA256 == "" {
		return nil, errors.New("tool runtime identity or context is incomplete")
	}
	configuration, err := toolRuntimeConfiguration(args, environment, modelPath, projectorPath, port, info.TemplateSHA256)
	if err != nil {
		return nil, err
	}
	if executablePath == "" {
		return nil, errors.New("owned runtime executable path is unavailable")
	}
	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return nil, err
	}
	modelPath, err = filepath.Abs(modelPath)
	if err != nil {
		return nil, err
	}
	if projectorPath != "" {
		projectorPath, err = filepath.Abs(projectorPath)
		if err != nil {
			return nil, err
		}
	}
	return &ToolRuntimeObservation{ModelID: model, Generation: generation, ConfigSHA256: configuration, Info: *info, ExecutablePath: executablePath, ModelPath: modelPath, ProjectorPath: projectorPath}, nil
}
