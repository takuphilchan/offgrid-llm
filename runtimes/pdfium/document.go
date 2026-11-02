package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
)

type pdfDocument struct {
	engine                 *pdfEngine
	pointer, handle, pages uint64
	cache                  wazero.CompilationCache
}

func (d *pdfDocument) close() {
	if d.handle != 0 {
		_, _ = d.engine.call("FPDF_CloseDocument", d.handle)
	}
	if d.pointer != 0 {
		_, _ = d.engine.call("free", d.pointer)
	}
	d.engine.close()
	if d.cache != nil {
		_ = d.cache.Close(d.engine.ctx)
	}
}

func openPDF(ctx context.Context, data []byte) (*pdfDocument, string) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return nil, "document_invalid"
	}
	if len(data) > maxInput {
		return nil, "document_limit"
	}
	config := wazero.NewRuntimeConfig().WithMemoryLimitPages(8192).WithCloseOnContextDone(true)
	var ownedCache wazero.CompilationCache
	if path := os.Getenv("OFFGRID_DOCUMENT_CODE_CACHE"); filepath.IsAbs(path) {
		if err := os.MkdirAll(path, 0700); err == nil {
			if info, err := os.Lstat(path); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				if cache, err := wazero.NewCompilationCacheWithDir(path); err == nil {
					ownedCache = cache
					config = config.WithCompilationCache(cache)
				}
			}
		}
	}
	e, err := newPDFEngine(ctx, config)
	if err != nil {
		if ownedCache != nil {
			_ = ownedCache.Close(ctx)
		}
		return nil, "document_parser_unavailable"
	}
	d := &pdfDocument{engine: e, cache: ownedCache}
	ready := false
	defer func() {
		if !ready {
			d.close()
		}
	}()
	d.pointer, err = e.allocate(uint64(len(data)))
	if err != nil || !e.module.Memory().Write(uint32(d.pointer), data) {
		return nil, "document_limit"
	}
	d.handle, err = e.call("FPDF_LoadMemDocument64", d.pointer, uint64(len(data)), 0)
	if err != nil || d.handle == 0 {
		code, _ := e.call("FPDF_GetLastError")
		if code == 4 || code == 5 {
			return nil, "document_encrypted"
		}
		return nil, "document_invalid"
	}
	d.pages, err = e.call("FPDF_GetPageCount", d.handle)
	if err != nil || int32(d.pages) < 1 {
		return nil, "document_empty"
	}
	if d.pages > maxPages {
		return nil, "document_limit"
	}
	ready = true
	return d, ""
}
