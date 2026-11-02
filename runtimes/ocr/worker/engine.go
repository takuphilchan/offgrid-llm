package main

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed assets/ocr.wasm
var engineBytes []byte

//go:embed assets/NOTICE.txt
var engineNotices string

// Filled only from the reproducible source build and then independently tested.
const engineSHA = "acfa0590d55b0762fc9d82fbafa6b13fa8700613356bfdf94cf933497dc298cf"

type ocrEngine struct {
	ctx     context.Context
	runtime wazero.Runtime
	module  api.Module
	handle  uint64
}

func newEngine(ctx context.Context, model []byte) (*ocrEngine, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if digest(engineBytes) != engineSHA {
		return nil, fmt.Errorf("OCR engine digest mismatch")
	}
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(8192).WithCloseOnContextDone(true))
	fail := func(err error) (*ocrEngine, error) { _ = r.Close(ctx); return nil, err }
	compiled, err := r.CompileModule(ctx, engineBytes)
	if err != nil {
		return fail(err)
	}
	// WASI has no filesystem mounts, environment, network, inherited handles or
	// diagnostic output. Reject unreviewed imports even if WASI could supply them.
	allowed := map[string]bool{"environ_get": true, "environ_sizes_get": true, "fd_write": true, "fd_read": true, "fd_close": true, "fd_seek": true, "clock_time_get": true}
	env := r.NewHostModuleBuilder("env")
	for _, f := range compiled.ImportedFunctions() {
		namespace, name, _ := f.Import()
		if namespace == "env" {
			switch name {
			case "emscripten_notify_memory_growth":
				env.NewFunctionBuilder().WithFunc(func(uint32) {}).Export(name)
			case "__syscall_getcwd", "__syscall_unlinkat", "__syscall_rmdir":
				// No host path discovery or file mutations. These libc imports are
				// retained by the static library but are never granted host access.
				env.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) { stack[0] = uint64(uint32(0xffffffc1)) }), f.ParamTypes(), f.ResultTypes()).Export(name)
			default:
				return fail(fmt.Errorf("unreviewed OCR import %s.%s", namespace, name))
			}
			continue
		}
		if namespace != "wasi_snapshot_preview1" || !allowed[name] {
			return fail(fmt.Errorf("unreviewed OCR import %s.%s", namespace, name))
		}
	}
	if _, err = env.Instantiate(ctx); err != nil {
		return fail(err)
	}
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return fail(err)
	}
	m, err := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStartFunctions("_initialize"))
	if err != nil {
		return fail(err)
	}
	e := &ocrEngine{ctx: ctx, runtime: r, module: m}
	ptr, err := e.write(model)
	if err != nil {
		return fail(err)
	}
	lang, err := e.write([]byte("eng\x00"))
	if err != nil {
		return fail(err)
	}
	e.handle, err = e.call("offgrid_ocr_create", ptr, uint64(len(model)), lang)
	if err != nil || e.handle == 0 {
		return fail(fmt.Errorf("OCR language initialization failed"))
	}
	// Model bytes remain allocated for the module lifetime: the engine may retain
	// references after initialization. Each request gets a fresh module.
	return e, nil
}

func (e *ocrEngine) call(name string, args ...uint64) (uint64, error) {
	f := e.module.ExportedFunction(name)
	if f == nil {
		return 0, fmt.Errorf("OCR export unavailable")
	}
	values, err := f.Call(e.ctx, args...)
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	return values[0], nil
}
func (e *ocrEngine) write(data []byte) (uint64, error) {
	ptr, err := e.call("malloc", uint64(len(data)))
	if err != nil || ptr == 0 {
		return 0, fmt.Errorf("OCR allocation failed")
	}
	if !e.module.Memory().Write(uint32(ptr), data) {
		return 0, fmt.Errorf("OCR memory bounds")
	}
	return ptr, nil
}
func (e *ocrEngine) recognize(pixels []byte, width, height int) (string, error) {
	ptr, err := e.write(pixels)
	if err != nil {
		return "", err
	}
	defer e.call("free", ptr)
	result, err := e.call("offgrid_ocr_recognize", e.handle, ptr, uint64(len(pixels)), uint64(width), uint64(height))
	if err != nil || result == 0 {
		return "", fmt.Errorf("OCR recognition failed")
	}
	defer e.call("offgrid_ocr_free_result", result)
	size, err := e.call("offgrid_ocr_result_size", result)
	if err != nil || size == 0 || size > 2<<20 {
		return "", fmt.Errorf("OCR output limit")
	}
	data, ok := e.module.Memory().Read(uint32(result), uint32(size))
	if !ok {
		return "", fmt.Errorf("OCR result bounds")
	}
	return string(data), nil
}
func (e *ocrEngine) close() {
	_, _ = e.call("offgrid_ocr_destroy", e.handle)
	_ = e.runtime.Close(context.Background())
}
