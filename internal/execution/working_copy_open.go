package execution

import (
	"io/fs"
	"os"
	"strings"
)

// Root.OpenFile follows in-root symbolic links, even with caller-supplied
// O_NOFOLLOW on Unix. Open each component relative to a retained directory
// handle instead, then compare the opened object with the observed input.
func openWorkingCopyObject(root *os.Root, name string, expected os.FileInfo, writable bool) (*os.File, error) {
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || len(name) > 4096 || strings.Count(name, "/") > 128 {
		return nil, ErrPath
	}
	if expected == nil || (!expected.Mode().IsRegular() && !expected.IsDir()) || expected.Mode()&os.ModeSymlink != 0 || (writable && expected.IsDir()) {
		return nil, ErrUnsupported
	}
	f, err := openWorkingCopyRooted(root, name, writable)
	if err != nil {
		return nil, err
	}
	actual, err := f.Stat()
	if err != nil || actual.Mode().Type() != expected.Mode().Type() || !os.SameFile(expected, actual) || (!actual.IsDir() && (actual.Size() != expected.Size() || !actual.ModTime().Equal(expected.ModTime()))) {
		f.Close()
		return nil, ErrRevision
	}
	return f, nil
}
