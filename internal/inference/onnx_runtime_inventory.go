//go:build windows || linux

package inference

import (
	"context"
	"errors"
	"io"
	"path/filepath"
)

// A Python distribution has more files than the Ollama bundle. The count and
// aggregate byte caps are explicit; directory traversal never reads all entries.
func (lease *managedOllamaFiles) onnxRuntimeTree(ctx context.Context, root string, pins map[string]string) error {
	count, found := 0, 0
	var total int64
	var walk func(string, bool, int) error
	walk = func(path string, directory bool, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 20000 || depth > 32 {
			return errors.New("ONNX runtime inventory exceeds entry bounds")
		}
		if err := checkONNXRuntimePath(path, directory); err != nil {
			return err
		}
		file, err := lease.open(path, directory)
		if err != nil {
			return err
		}
		if directory {
			entries, err := file.ReadDir(20001 - count)
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if len(entries) > 20000-count {
				return errors.New("ONNX runtime inventory exceeds entry bounds")
			}
			for _, entry := range entries {
				if err := walk(filepath.Join(path, entry.Name()), entry.IsDir(), depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		pin := pins[filepath.ToSlash(relative)]
		if pin == "" {
			return errors.New("ONNX runtime contains an unpinned file")
		}
		info, err := file.Stat()
		if err != nil {
			return err
		}
		if info.Size() < 0 || info.Size() > 512<<20 || total+info.Size() > 4<<30 {
			return errors.New("ONNX runtime inventory exceeds byte bounds")
		}
		total += info.Size()
		if err := managedHash(ctx, file, pin, 512<<20); err != nil {
			return err
		}
		if err := validateONNXRuntimeImportPath(root, path, file); err != nil {
			return err
		}
		found++
		return nil
	}
	if err := walk(root, true, 0); err != nil {
		return err
	}
	if found != len(pins) {
		return errors.New("ONNX runtime inventory has missing files")
	}
	return nil
}
