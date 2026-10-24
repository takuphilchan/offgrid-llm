package verification

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/platform"
)

type entry struct {
	digest    [32]byte
	size      int64
	mode      fs.FileMode
	directory bool
}

type snapshot map[string]entry

type workspace struct {
	root     *os.Root
	path     string
	identity fs.FileInfo
}

func openWorkspace(path string) (*workspace, error) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, errors.New("host verification is unsupported on this platform")
	}
	path = filepath.Clean(path)
	if filepath.Dir(path) == path || strings.HasPrefix(filepath.ToSlash(path), "//") {
		return nil, errors.New("verification requires a bounded local workspace")
	}
	if err := noLinkedParents(path); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, errors.New("cannot open verification workspace")
	}
	info, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, errors.New("cannot inspect verification workspace")
	}
	w := &workspace{root: root, path: path, identity: info}
	if err := w.checkIdentity(); err != nil {
		root.Close()
		return nil, err
	}
	return w, nil
}

func noLinkedParents(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || ((!info.IsDir() || info.Mode()&os.ModeSymlink != 0) && !platform.IsSystemPathAlias(current)) {
			return errors.New("verification workspace must not traverse links or non-directories")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func (w *workspace) checkIdentity() error {
	if err := noLinkedParents(w.path); err != nil {
		return err
	}
	current, err := os.Stat(w.path)
	if err != nil || !os.SameFile(current, w.identity) {
		return errors.New("verification workspace identity changed")
	}
	return nil
}

func (w *workspace) close() error { return w.root.Close() }

// capture is bounded and read-only. os.Root contains traversal across concurrent
// symlink swaps. Regular-file identity is checked before reading and afterwards;
// linked/special files fail closed. This is not an adversarial filesystem sandbox
// or an atomic filesystem snapshot (e.g. it cannot prove no transient changes).
func (w *workspace) capture(ctx context.Context) (snapshot, error) {
	if err := w.checkIdentity(); err != nil {
		return nil, err
	}
	rootInfo, err := w.root.Lstat(".")
	if err != nil {
		return nil, errors.New("cannot inspect verification workspace")
	}
	result := snapshot{".": {directory: true, mode: rootInfo.Mode()}}
	var total int64
	var walk func(string, int) error
	walk = func(path string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 16 {
			return errors.New("verification directory depth exceeds limit")
		}
		before, err := w.root.Lstat(path)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe verification directory")
		}
		dir, err := openRead(w.root, path)
		if err != nil {
			return errors.New("cannot read verification directory")
		}
		opened, statErr := dir.Stat()
		if statErr != nil || !os.SameFile(before, opened) {
			dir.Close()
			return errors.New("verification directory changed")
		}
		children, err := dir.ReadDir(maxEntries - len(result) + 1)
		dir.Close()
		if err != nil && err != io.EOF {
			return errors.New("cannot enumerate verification directory")
		}
		if len(result)+len(children) > maxEntries {
			return errors.New("verification workspace entry limit exceeded")
		}
		// Reserve every child before descending, keeping the global entry bound.
		for _, child := range children {
			name := child.Name()
			if path != "." {
				name = path + "/" + name
			}
			if !validPath(name) {
				return errors.New("unsupported verification path")
			}
			info, err := w.root.Lstat(name)
			if err != nil {
				return errors.New("verification entry changed")
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return errors.New("verification rejects links and special files")
			}
			result[name] = entry{size: info.Size(), mode: info.Mode(), directory: info.IsDir()}
		}
		for _, child := range children {
			name := child.Name()
			if path != "." {
				name = path + "/" + name
			}
			item := result[name]
			if item.directory {
				item.size = 0
				result[name] = item
				if err := walk(name, depth+1); err != nil {
					return err
				}
			} else {
				item, err = w.read(ctx, name, maxTotalBytes-total)
				if err != nil {
					return err
				}
				total += item.size
				result[name] = item
			}
		}
		after, err := w.root.Lstat(path)
		if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode() {
			return errors.New("verification directory changed during capture")
		}
		return nil
	}
	if err := walk(".", 0); err != nil {
		return nil, err
	}
	if err := w.checkIdentity(); err != nil {
		return nil, err
	}
	return result, nil
}

func (w *workspace) read(ctx context.Context, path string, budget int64) (entry, error) {
	if err := ctx.Err(); err != nil {
		return entry{}, err
	}
	before, err := w.root.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return entry{}, errors.New("verification requires a regular file")
	}
	if before.Size() > maxFileBytes || before.Size() > budget {
		return entry{}, errors.New("verification file/byte limit exceeded")
	}
	f, err := openRead(w.root, path)
	if err != nil {
		return entry{}, errors.New("cannot open verification file")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() || !singleLink(f) {
		return entry{}, errors.New("verification file identity or link count is unsafe")
	}
	hash := sha256.New()
	// Never ReadAll: content is neither retained nor written into evidence.
	n, err := io.Copy(hash, io.LimitReader(f, min(int64(maxFileBytes), budget)+1))
	if err != nil || n > maxFileBytes || n > budget {
		return entry{}, errors.New("verification file read exceeds bounds")
	}
	if err := ctx.Err(); err != nil {
		return entry{}, err
	}
	after, err := f.Stat()
	current, pathErr := w.root.Lstat(path)
	if err != nil || pathErr != nil || !current.Mode().IsRegular() || !os.SameFile(before, current) || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Size() != n || after.Size() != n || before.Mode() != after.Mode() || !singleLink(f) {
		return entry{}, errors.New("verification file changed during read")
	}
	item := entry{size: n, mode: after.Mode()}
	copy(item.digest[:], hash.Sum(nil))
	return item, nil
}

func (v *Verifier) capture(ctx context.Context, allowDestination bool) (snapshot, error) {
	after, err := v.workspace.capture(ctx)
	if err != nil {
		return nil, err
	}
	for name, old := range v.baseline {
		if allowDestination && name == v.contract.Destination {
			continue
		}
		if current, ok := after[name]; !ok || current != old {
			return nil, errors.New("verification workspace contents changed")
		}
	}
	for name := range after {
		if _, ok := v.baseline[name]; !ok && !(allowDestination && name == v.contract.Destination) {
			return nil, errors.New("verification found an unexpected workspace entry")
		}
	}
	return after, nil
}
