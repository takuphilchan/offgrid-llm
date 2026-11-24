package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

type WorkingCopyEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Bytes  int64  `json:"bytes"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256,omitempty"`
}

type WorkingCopyExclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type WorkingCopyManifest struct {
	Version              int                    `json:"version"`
	Source               Binding                `json:"source"`
	InstructionVersion   uint64                 `json:"instruction_version"`
	ControlEpoch         uint64                 `json:"control_epoch"`
	Entries              []WorkingCopyEntry     `json:"entries"`
	Exclusions           []WorkingCopyExclusion `json:"exclusions"`
	Bytes                int64                  `json:"bytes"`
	Prerequisites        []ChangeSetReference   `json:"prerequisites,omitempty"`
	AppliedPrerequisites []ChangeSetReference   `json:"applied_prerequisites,omitempty"`
}

// Inventory includes the actual working tree, independent of Git tracking or
// ignore rules. Git administration is explicitly excluded; a linked-worktree
// .git file must never point a worker back at the user's index or object store.
func inventoryWorkingCopy(ctx context.Context, source *os.Root, req WorkingCopyRequest, policy WorkingCopyPolicy) (*WorkingCopyManifest, error) {
	m := &WorkingCopyManifest{Version: 1, Source: req.Parent, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch, Entries: []WorkingCopyEntry{}, Exclusions: []WorkingCopyExclusion{}}
	// Included entries and excluded Git roots retain their separate admission
	// budgets; the walker bounds their combined directory allocation up front.
	err := walkWorkingCopy(ctx, source, 2*policy.MaxEntries, func(name string, info os.FileInfo) error {
		if path.Base(name) == ".git" {
			m.Exclusions = append(m.Exclusions, WorkingCopyExclusion{name, "Git administration is excluded; all ordinary working-tree files are inventoried regardless of tracking or ignore status"})
			if len(m.Exclusions) > policy.MaxEntries {
				return ErrLimit
			}
			if info.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if info.Mode().Type() != 0 && info.Mode().Type() != os.ModeDir {
			return fmt.Errorf("working copy preparation does not support links or special files: %s", name)
		}
		if len(m.Entries) >= policy.MaxEntries {
			return ErrLimit
		}
		item := WorkingCopyEntry{Path: name, Kind: "directory", Mode: uint32(info.Mode().Perm())}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || info.Size() > req.MaxBytes-m.Bytes {
				return ErrLimit
			}
			item.Kind, item.Bytes = "file", info.Size()
			var err error
			item.SHA256, err = hashWorkingCopyFile(ctx, source, name, info)
			if err != nil {
				return err
			}
			m.Bytes += item.Bytes
		}
		m.Entries = append(m.Entries, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := requireWorkingCopyPaths(m, req.RequiredPaths); err != nil {
		return nil, err
	}
	return m, nil
}

func requireWorkingCopyPaths(m *WorkingCopyManifest, requiredPaths []string) error {
	for _, required := range requiredPaths {
		found := required == "."
		for _, entry := range m.Entries {
			if entry.Path == required {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("required working-copy input is absent or excluded: %s", required)
		}
	}
	return nil
}

type copyContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r copyContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func openWorkingCopyFile(source *os.Root, name string, expected os.FileInfo) (*os.File, error) {
	if expected == nil || !expected.Mode().IsRegular() {
		return nil, ErrUnsupported
	}
	return openWorkingCopyObject(source, name, expected, false)
}

func hashWorkingCopyFile(ctx context.Context, source *os.Root, name string, expected os.FileInfo) (string, error) {
	f, err := openWorkingCopyFile(source, name, expected)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(copyContextReader{ctx, f}, expected.Size()+1))
	if err != nil {
		return "", err
	}
	after, err := f.Stat()
	if err != nil || n != expected.Size() || after.Size() != expected.Size() || !after.ModTime().Equal(expected.ModTime()) {
		return "", ErrRevision
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func workingCopyManifestDigest(m *WorkingCopyManifest) (string, []byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}
