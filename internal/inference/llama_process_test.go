package inference

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type llamaProcessCacheFixture struct {
	actions   *[]string
	context   int
	path      string
	projector string
	ids       []string
	port      int
	unloadErr error
	onLoad    func()
}

func (f *llamaProcessCacheFixture) SetContextSize(size int) {
	f.context = size
	*f.actions = append(*f.actions, "context")
}
func (f *llamaProcessCacheFixture) GetOrLoadContext(ctx context.Context, id, path, projector string) (*ModelInstance, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.path, f.projector = path, projector
	f.ids = []string{id}
	*f.actions = append(*f.actions, "cache-load")
	if f.onLoad != nil {
		f.onLoad()
	}
	return &ModelInstance{ModelID: id, Port: f.port}, nil
}
func (f *llamaProcessCacheFixture) IsModelAlive(id string) bool {
	return len(f.ids) == 1 && f.ids[0] == id
}
func (f *llamaProcessCacheFixture) InstanceGeneration(id string) uint64 {
	if f.IsModelAlive(id) {
		return 1
	}
	return 0
}
func (f *llamaProcessCacheFixture) LoadedModelIDs() []string { return append([]string(nil), f.ids...) }
func (f *llamaProcessCacheFixture) Unload(id string) error {
	*f.actions = append(*f.actions, "cache-unload")
	if f.unloadErr != nil {
		return f.unloadErr
	}
	f.ids = nil
	return nil
}

type llamaProcessEngineFixture struct {
	Engine
	actions *[]string
	port    int
	loaded  bool
	loadErr error
}

func (f *llamaProcessEngineFixture) SetPort(port int) {
	f.port = port
	*f.actions = append(*f.actions, "port")
}
func (f *llamaProcessEngineFixture) Load(ctx context.Context, _ string, _ LoadOptions) error {
	*f.actions = append(*f.actions, "engine-load")
	if f.loadErr != nil {
		return f.loadErr
	}
	f.loaded = true
	return ctx.Err()
}
func (f *llamaProcessEngineFixture) Unload() error {
	*f.actions = append(*f.actions, "engine-unload")
	f.loaded = false
	return nil
}
func (f *llamaProcessEngineFixture) IsLoaded() bool { return f.loaded }

func TestLlamaCPPProcessPreservesLoadAndRetirementOrder(t *testing.T) {
	var actions []string
	cache := &llamaProcessCacheFixture{actions: &actions, port: 12345}
	engine := &llamaProcessEngineFixture{Engine: NewMockEngine(), actions: &actions}
	process := &LlamaCPPProcess{cache: cache, engine: engine}
	port, err := process.Activate(context.Background(), "model", "model.gguf", "projector.gguf", LoadOptions{ContextSize: 8192, NumThreads: 4})
	if err != nil || port != 12345 || cache.context != 8192 || cache.path != "model.gguf" || cache.projector != "projector.gguf" || engine.port != 12345 || !process.Alive("model") {
		t.Fatalf("llama activation changed cache/engine contract: port=%d err=%v cache=%+v engine=%+v", port, err, cache, engine)
	}
	if want := []string{"context", "cache-load", "port", "engine-load"}; !reflect.DeepEqual(actions, want) {
		t.Fatalf("llama activation order = %v, want %v", actions, want)
	}
	retired, err := process.RetireAll(context.Background())
	if err != nil || !retired || process.Alive("model") || engine.IsLoaded() {
		t.Fatalf("llama process remained resident: retired=%v err=%v", retired, err)
	}
	if want := []string{"context", "cache-load", "port", "engine-load", "cache-unload", "engine-unload"}; !reflect.DeepEqual(actions, want) {
		t.Fatalf("llama retirement order = %v, want %v", actions, want)
	}
}

func TestLlamaCPPProcessRetirementFailsClosedOnPartialUnload(t *testing.T) {
	var actions []string
	failure := errors.New("unload failed")
	cache := &llamaProcessCacheFixture{actions: &actions, ids: []string{"model"}, unloadErr: failure}
	engine := &llamaProcessEngineFixture{Engine: NewMockEngine(), actions: &actions, loaded: true}
	process := &LlamaCPPProcess{cache: cache, engine: engine}
	retired, err := process.RetireAll(context.Background())
	if !retired || !errors.Is(err, failure) || !engine.IsLoaded() || !reflect.DeepEqual(actions, []string{"cache-unload"}) {
		t.Fatalf("failed unload was treated as clean retirement: retired=%v err=%v actions=%v", retired, err, actions)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if retired, err := process.RetireAll(ctx); retired || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled retirement had effects: retired=%v err=%v", retired, err)
	}
}

func TestLlamaCPPProcessStopAfterCacheLoadDoesNotConnectEngine(t *testing.T) {
	var actions []string
	ctx, cancel := context.WithCancel(context.Background())
	cache := &llamaProcessCacheFixture{actions: &actions, port: 12345, onLoad: cancel}
	engine := &llamaProcessEngineFixture{Engine: NewMockEngine(), actions: &actions}
	process := &LlamaCPPProcess{cache: cache, engine: engine}
	port, err := process.Activate(ctx, "model", "model.gguf", "", LoadOptions{ContextSize: 4096})
	if port != 0 || !errors.Is(err, context.Canceled) || engine.IsLoaded() || !reflect.DeepEqual(actions, []string{"context", "cache-load"}) {
		t.Fatalf("cancelled load connected engine: port=%d err=%v actions=%v", port, err, actions)
	}
}
