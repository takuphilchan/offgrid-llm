package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrHostProtectionConflict  = errors.New("host_protection_conflict")
	ErrHostProtectionChanged   = errors.New("host_protection_identity_changed")
	ErrHostProtectionUncertain = errors.New("host_protection_uncertain")
	ErrHostProtectionCorrupt   = errors.New("host_protection_corrupt")
	ErrHostProtectionBusy      = errors.New("host_protection_busy")
	ErrHostProtectionLimit     = errors.New("host_protection_limit")
	ErrHostProtectionStorage   = errors.New("host_protection_storage_failed")
)

const hostProtectionMaxRecords = 1024

// HostProtectionRegistry coordinates trusted native supervisors for one OS
// account, independent of desktop profiles or service workspaces. It is an
// admission journal, not an executor, consent grant or process sandbox.
type HostProtectionRegistry struct {
	directory string
	ancestry  func(string) error
}

func (s *HostProtectionRegistry) checkAncestry(path string) error {
	if s.ancestry != nil {
		return s.ancestry(path)
	}
	return CheckPrivateAncestry(path)
}

// AccountHostProtectionRegistry resolves the existing account namespace without
// reading or changing it. There is no renderer-controlled registry location.
func AccountHostProtectionRegistry() (*HostProtectionRegistry, error) {
	identity, err := ProjectIdentityDirectory()
	if err != nil {
		return nil, err
	}
	return &HostProtectionRegistry{directory: filepath.Join(filepath.Dir(identity), "host-protection")}, nil
}

type hostProtectionLocation struct {
	Path     string `json:"path"`
	Anchor   string `json:"anchor"`
	Identity string `json:"identity"`
	Suffix   string `json:"suffix"`
}

type hostProtectionRecord struct {
	Version        int                     `json:"version"`
	ID             string                  `json:"id"`
	Kind           string                  `json:"kind"`
	State          string                  `json:"state"`
	Root           hostProtectionLocation  `json:"root"`
	Journal        *hostProtectionLocation `json:"journal,omitempty"`
	Binding        *Binding                `json:"binding,omitempty"`
	Backend        string                  `json:"backend,omitempty"`
	Permissions    Permissions             `json:"permissions,omitempty"`
	CapabilityHash string                  `json:"capability_hash,omitempty"`
}

// ProtectedBrowserRoot is an in-memory capability for finalizing a reservation
// made before its directory existed. It cannot be reconstructed from public
// metadata, a saved row, a PID or a subsequent registration of the same path.
type ProtectedBrowserRoot struct {
	registry   *HostProtectionRegistry
	record     hostProtectionRecord
	capability string
}

func (r *ProtectedBrowserRoot) ID() string {
	if r == nil {
		return ""
	}
	return r.record.ID
}

// HostProtectionLease retains authority to publish verified cleanup. Runtime
// wiring must supply its own typed cleanup owner; there is intentionally no
// exported boolean-proof release API and no PID/TTL-based release operation.
type HostProtectionLease struct {
	registry   *HostProtectionRegistry
	id         string
	capability string
	record     hostProtectionRecord
	configHash string
	mu         sync.Mutex
	activated  bool
	released   bool
}

func (l *HostProtectionLease) ID() string {
	if l == nil {
		return ""
	}
	return l.id
}

func protectionHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *HostProtectionRegistry) validateLocation(path string, existing bool) (hostProtectionLocation, error) {
	location, err := locateHostProtection(path)
	if err != nil {
		return location, err
	}
	if existing && location.Suffix != "" {
		return location, ErrPath
	}
	if s == nil {
		return location, ErrAuthority
	}
	registry, err := locateHostProtection(s.directory)
	if err != nil {
		return location, err
	}
	if protectionOverlap(registry.Path, location.Path) {
		return location, ErrHostProtectionConflict
	}
	return location, nil
}

// Finalize pins the newly created root only with the original pending
// reservation capability and unchanged ancestor. Crash recovery never calls
// this with a freshly generated capability or adopts whatever now exists.
func (r *ProtectedBrowserRoot) Finalize(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || r.registry == nil {
		return ErrAuthority
	}
	if r.record.State == "protected" {
		current, err := locateHostProtection(r.record.Root.Path)
		if err != nil || current != r.record.Root {
			return ErrHostProtectionChanged
		}
		return nil
	}
	if !validCredential(r.capability) {
		return ErrAuthority
	}
	ancestor, err := folderIdentity(r.record.Root.Anchor)
	if err != nil || ancestor != r.record.Root.Identity {
		return ErrHostProtectionChanged
	}
	current, err := locateHostProtection(r.record.Root.Path)
	if err != nil || current.Suffix != "" || !protectionSamePath(current.Path, r.record.Root.Path) {
		return ErrHostProtectionChanged
	}
	if CheckPrivatePath(current.Path) != nil {
		return ErrAuthority
	}
	return r.registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		row, ok := store.records[r.record.ID]
		if !ok || row.Kind != "browser" || row.CapabilityHash != protectionHash(r.capability) {
			return ErrAuthority
		}
		if row.State == "protected" {
			if row.Root != current {
				return ErrHostProtectionChanged
			}
			return nil
		}
		if row.State != "pending" || row.Root != r.record.Root {
			return ErrHostProtectionChanged
		}
		actual, err := folderIdentity(row.Root.Anchor)
		if err != nil || actual != row.Root.Identity {
			return ErrHostProtectionChanged
		}
		observed, err := locateHostProtection(current.Path)
		if err != nil || observed != current {
			return ErrHostProtectionChanged
		}
		row.State = "protected"
		row.Root = current
		return store.replace(ctx, row)
	})
}

// releaseVerified is intentionally package-private until the existing worker's
// cleanup owner is wired to it. The hash in durable state alone cannot release
// a lease. Repeating a release with the original capability is idempotent.
func (l *HostProtectionLease) releaseVerified(ctx context.Context) error {
	if l == nil || l.registry == nil || !validCredential(l.capability) {
		return ErrAuthority
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.publishRelease(ctx); err != nil {
		return err
	}
	l.released = true
	return nil
}

// CancelUnactivated is only available on the original native reservation. The
// worker activation boundary and this cancellation serialize on the same mutex;
// once activation wins, only verified worker cleanup may release the row.
func (l *HostProtectionLease) CancelUnactivated(ctx context.Context) error {
	if l == nil || l.registry == nil || !validCredential(l.capability) {
		return ErrAuthority
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.activated {
		return ErrAuthority
	}
	if err := l.publishRelease(ctx); err != nil {
		return err
	}
	l.released = true
	return nil
}

func (l *HostProtectionLease) publishRelease(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// The original owner remembers successful durable settlement. Its bounded
	// diagnostic tombstone may subsequently be reclaimed; retrying is a no-op,
	// never an authorization to release a different or freshly admitted lease.
	if l.released {
		return nil
	}
	return l.registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		row, ok := store.records[l.id]
		if !ok || row.Kind != "execution" || row.CapabilityHash != protectionHash(l.capability) {
			return ErrAuthority
		}
		if row.Root != l.record.Root || row.Journal == nil || l.record.Journal == nil || *row.Journal != *l.record.Journal ||
			row.Binding == nil || l.record.Binding == nil || *row.Binding != *l.record.Binding || row.Backend != l.record.Backend || row.Permissions != l.record.Permissions {
			return ErrHostProtectionChanged
		}
		store.afterCommit = func() { l.released = true }
		if row.State == "released" {
			return nil
		}
		if row.State != "reserved" {
			return ErrHostProtectionCorrupt
		}
		row.State = "released"
		return store.replace(ctx, row)
	})
}

func protectionPrivateFile(path string, optional bool) error {
	info, err := os.Lstat(path)
	if optional && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return ErrHostProtectionCorrupt
	}
	return CheckPrivatePath(path)
}
