package main

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
)

// This release uses Emscripten's legalized (split i64) ABI, including fd_seek.
// It must not be linked to a standard WASI signature or to host file operations.
func instantiateImports(ctx context.Context, r wazero.Runtime, compiled wazero.CompiledModule) error {
	env := r.NewHostModuleBuilder("env")
	exporter, err := emscripten.NewFunctionExporterForModule(compiled)
	if err != nil {
		return err
	}
	exporter.ExportFunctions(env)
	wasi := r.NewHostModuleBuilder("wasi_snapshot_preview1")
	for _, f := range compiled.ImportedFunctions() {
		module, name, _ := f.Import()
		var handler api.GoModuleFunc
		builder := env
		switch module {
		case "env":
			switch name {
			case "invoke_viii", "invoke_ii", "invoke_iii", "invoke_iiii", "invoke_iiiii", "_emscripten_throw_longjmp":
				continue // Implemented by wazero's Emscripten exporter.
			case "emscripten_resize_heap":
				handler = resizeHeap
			case "_emscripten_memcpy_js":
				handler = copyMemory
			case "emscripten_date_now":
				handler = func(_ context.Context, _ api.Module, stack []uint64) { stack[0] = api.EncodeF64(0) }
			case "_tzset_js":
				handler = utcTimezone
			case "emscripten_err":
				handler = func(context.Context, api.Module, []uint64) {} // No document diagnostics leave the module.
			case "_abort_js", "_localtime_js", "_gmtime_js":
				handler = func(context.Context, api.Module, []uint64) { panic("unsupported PDF runtime operation") }
			case "__syscall_openat", "__syscall_fcntl64", "__syscall_ioctl", "__syscall_fstat64", "__syscall_stat64", "__syscall_newfstatat", "__syscall_lstat64", "__syscall_getdents64", "__syscall_unlinkat", "__syscall_rmdir", "__syscall_ftruncate64", "_mmap_js", "_munmap_js":
				handler = func(_ context.Context, _ api.Module, stack []uint64) { stack[0] = uint64(uint32(0xffffffc1)) } // -EPERM in Emscripten's errno ABI.
			default:
				return fmt.Errorf("unreviewed PDF environment import %s", name)
			}
		case "wasi_snapshot_preview1":
			builder = wasi
			switch name {
			case "environ_sizes_get":
				handler = emptyEnvironment
			case "environ_get":
				handler = func(_ context.Context, _ api.Module, stack []uint64) { stack[0] = 0 }
			case "fd_write", "fd_read", "fd_close", "fd_sync", "fd_seek":
				handler = func(_ context.Context, _ api.Module, stack []uint64) { stack[0] = 8 } // EBADF: no file descriptors are exposed.
			default:
				return fmt.Errorf("unreviewed PDF WASI import %s", name)
			}
		default:
			return fmt.Errorf("unreviewed PDF import namespace %s", module)
		}
		builder.NewFunctionBuilder().WithGoModuleFunction(handler, f.ParamTypes(), f.ResultTypes()).Export(name)
	}
	if _, err := wasi.Instantiate(ctx); err != nil {
		return err
	}
	_, err = env.Instantiate(ctx)
	return err
}

func resizeHeap(_ context.Context, m api.Module, stack []uint64) {
	requested := uint64(uint32(stack[0]))
	old, _ := m.Memory().Grow(0)
	stack[0] = 0
	if requested > 512<<20 {
		return
	}
	if requested <= uint64(old)*65536 {
		stack[0] = 1
		return
	}
	_, ok := m.Memory().Grow(uint32((requested - uint64(old)*65536 + 65535) / 65536))
	if ok {
		stack[0] = 1
	}
}

func copyMemory(_ context.Context, m api.Module, stack []uint64) {
	src, ok := m.Memory().Read(uint32(stack[1]), uint32(stack[2]))
	if !ok {
		panic("PDF memory source out of bounds")
	}
	dst, ok := m.Memory().Read(uint32(stack[0]), uint32(stack[2]))
	if !ok {
		panic("PDF memory destination out of bounds")
	}
	copy(dst, src) // Go copy preserves overlapping memmove semantics.
}

func emptyEnvironment(_ context.Context, m api.Module, stack []uint64) {
	if !m.Memory().WriteUint32Le(uint32(stack[0]), 0) || !m.Memory().WriteUint32Le(uint32(stack[1]), 0) {
		panic("PDF environment pointers out of bounds")
	}
	stack[0] = 0
}

func utcTimezone(_ context.Context, m api.Module, stack []uint64) {
	for _, ptr := range stack[:2] {
		if !m.Memory().WriteUint32Le(uint32(ptr), 0) {
			panic("PDF timezone pointer out of bounds")
		}
	}
	for _, ptr := range stack[2:4] {
		if !m.Memory().Write(uint32(ptr), []byte{'U', 'T', 'C', 0}) {
			panic("PDF timezone name out of bounds")
		}
	}
}
