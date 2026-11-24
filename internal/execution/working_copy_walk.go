package execution

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

type workingCopyDirectoryReader interface {
	Readdir(int) ([]os.FileInfo, error)
}

// ReadDir(-1), including fs.WalkDir, allocates a whole untrusted directory
// before the visitor can enforce its entry budget. Reserve entries as batches
// are read; parent siblings and recursively enumerated children share the cap.
func readWorkingCopyDirectory(ctx context.Context, directory workingCopyDirectoryReader, remaining *int) ([]os.FileInfo, error) {
	var entries []os.FileInfo
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Readdir captures metadata relative to the open directory. Lazy
		// DirEntry.Info/root.Lstat can instead observe a substituted parent.
		batch, err := directory.Readdir(min(256, *remaining+1))
		if cancelErr := ctx.Err(); cancelErr != nil {
			return nil, cancelErr
		}
		if len(batch) > *remaining {
			return nil, ErrLimit
		}
		*remaining -= len(batch)
		entries = append(entries, batch...)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return nil, io.ErrNoProgress
		}
	}
	slices.SortFunc(entries, func(a, b os.FileInfo) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func walkWorkingCopy(ctx context.Context, root *os.Root, limit int, visit func(string, os.FileInfo) error) error {
	if limit < 0 {
		return ErrLimit
	}
	info, err := root.Stat(".")
	if err != nil {
		return err
	}
	remaining := limit
	var walk func(string, os.FileInfo) error
	walk = func(name string, expected os.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		directory, err := openWorkingCopyObject(root, name, expected, false)
		if err != nil {
			return err
		}
		if err = qualifyWorkingCopyDirectory(directory); err != nil {
			directory.Close()
			return err
		}
		entries, err := readWorkingCopyDirectory(ctx, directory, &remaining)
		closeErr := directory.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		for _, info := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			child := path.Join(name, info.Name())
			if len(child) > 4096 || strings.Count(child, "/") > 128 {
				return ErrLimit
			}
			if err = visit(child, info); err == fs.SkipDir {
				continue
			} else if err != nil {
				return err
			}
			if info.IsDir() {
				if err = walk(child, info); err != nil {
					return err
				}
			}
		}
		// Measurement must not accept a traversal whose path stopped naming
		// the pinned directory, even when all cached file sizes were readable.
		current, err := openWorkingCopyObject(root, name, expected, false)
		if err != nil {
			return err
		}
		if err = current.Close(); err != nil {
			return err
		}
		return ctx.Err()
	}
	return walk(".", info)
}
