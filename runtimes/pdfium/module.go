package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed assets/pdfium.wasm
var engineBytes []byte

//go:embed assets/NOTICE.txt
var engineNotices string

const engineSHA256 = "0001cb839e62425ba5b645c17300027708d916b5bfccf93c2cec7a8376dc3989"

// Each document gets a fresh module. The only imported capabilities are bounded
// memory operations, empty environment and explicit filesystem denial. No WASI
// host filesystem, networking, credentials, shell or image/font discovery exists.
type pdfEngine struct {
	ctx     context.Context
	runtime wazero.Runtime
	module  api.Module
}

func newPDFEngine(ctx context.Context, config wazero.RuntimeConfig) (*pdfEngine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(engineBytes)) != engineSHA256 {
		return nil, fmt.Errorf("bundled PDF engine digest mismatch")
	}
	r := wazero.NewRuntimeWithConfig(ctx, config)
	fail := func(err error) (*pdfEngine, error) { _ = r.Close(ctx); return nil, err }
	compiled, err := r.CompileModule(ctx, engineBytes)
	if err != nil {
		return fail(err)
	}
	if err = instantiateImports(ctx, r, compiled); err != nil {
		return fail(err)
	}
	m, err := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithStartFunctions("__wasm_call_ctors").WithFSConfig(wazero.NewFSConfig()))
	if err != nil {
		return fail(err)
	}
	e := &pdfEngine{ctx: ctx, runtime: r, module: m}
	if _, err = e.call("FPDF_InitLibrary"); err != nil {
		return fail(err)
	}
	return e, nil
}

func (e *pdfEngine) close() {
	_, _ = e.call("FPDF_DestroyLibrary")
	_ = e.runtime.Close(e.ctx)
}

func (e *pdfEngine) call(name string, args ...uint64) (uint64, error) {
	f := e.module.ExportedFunction(name)
	if f == nil {
		return 0, fmt.Errorf("PDF engine export unavailable")
	}
	result, err := f.Call(e.ctx, args...)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0], nil
}

func (e *pdfEngine) allocate(size uint64) (uint64, error) {
	ptr, err := e.call("malloc", size)
	if err != nil || ptr == 0 {
		return 0, fmt.Errorf("PDF memory allocation failed")
	}
	return ptr, nil
}
