package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const protectionRecoveryEnvironment = "OFFGRID_EXECUTOR_PROTECTION_RECOVERY"

// This envelope exists only on the original private startup pipe. It is never
// a WorkerIdentity, model receipt, inspection record or reconnect credential.
type protectionStartup struct {
	WorkerIdentity
	Protection *protectionHandoff `json:"host_protection,omitempty"`
}
type protectionHandoff struct {
	Version         int    `json:"version"`
	Lease           string `json:"lease"`
	Capability      string `json:"capability"`
	ConfigHash      string `json:"config_sha256"`
	JournalIdentity string `json:"journal_identity"`
}

type stdioProtectionOwner struct {
	mu              sync.Mutex
	lease           *HostProtectionLease
	journalIdentity string
}

func protectionConfigHash(config Context) string {
	data, _ := json.Marshal(config)
	return protectionHash(string(data))
}

func protectedStartup(w *Worker, lease *HostProtectionLease) (json.RawMessage, error) {
	if !nativeCleanupVerified(nil) || lease.record.Backend != "local" || !validCredential(lease.configHash) {
		return nil, ErrUnsupported
	}
	identity, err := protectionJournalIdentity(w.Journal.directory)
	if err != nil {
		return nil, err
	}
	return json.Marshal(protectionStartup{WorkerIdentity: workerIdentity(w.Version), Protection: &protectionHandoff{
		Version: 1, Lease: lease.id, Capability: lease.capability, ConfigHash: lease.configHash, JournalIdentity: identity,
	}})
}

func protectionJournalIdentity(directory string) (string, error) {
	path := filepath.Join(directory, "execution-dispatch.sqlite")
	if err := CheckPrivatePath(path); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return "", ErrHostProtectionChanged
	}
	return protectionRegularFileIdentity(path)
}

func (s *HostProtectionRegistry) acceptProtectionHandoff(ctx context.Context, handoff *protectionHandoff, config Context, directory string) (*stdioProtectionOwner, error) {
	if !nativeCleanupVerified(nil) || config.Backend != "local" || handoff == nil || handoff.Version != 1 ||
		!validCredential(handoff.Lease) || !validCredential(handoff.Capability) || handoff.ConfigHash != protectionConfigHash(config) || !validIdentity(handoff.JournalIdentity) {
		return nil, ErrAuthority
	}
	if err := s.checkAncestry(directory); err != nil {
		return nil, err
	}
	root, err := locateHostProtection(config.Root)
	if err != nil {
		return nil, err
	}
	journal, err := locateHostProtection(directory)
	if err != nil {
		return nil, err
	}
	identity, err := protectionJournalIdentity(directory)
	if err != nil || identity != handoff.JournalIdentity {
		return nil, ErrHostProtectionChanged
	}
	var lease *HostProtectionLease
	report, err := s.inspectRecords(ctx, "", 1, func(records map[string]hostProtectionRecord) error {
		row, ok := records[handoff.Lease]
		if !ok || row.Kind != "execution" || row.Backend != "local" || row.CapabilityHash != protectionHash(handoff.Capability) || row.Binding == nil ||
			*row.Binding != config.Binding || row.Permissions != config.Permissions || row.Root != root || row.Journal == nil || *row.Journal != journal {
			return ErrAuthority
		}
		lease = &HostProtectionLease{registry: s, id: row.ID, capability: handoff.Capability, record: row, configHash: handoff.ConfigHash, activated: true, released: row.State == "released"}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if report.Status != "ready" || lease == nil {
		return nil, ErrHostProtectionCorrupt
	}
	return &stdioProtectionOwner{lease: lease, journalIdentity: identity}, nil
}

func (p *stdioProtectionOwner) settle(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	// Successful publication survives bounded tombstone reclamation. Never
	// reopen missing state or touch a replacement row to repeat that success.
	p.lease.mu.Lock()
	released := p.lease.released
	p.lease.mu.Unlock()
	if released {
		return nil
	}
	directory := p.lease.record.Journal.Path
	if err := p.lease.registry.checkAncestry(directory); err != nil {
		return err
	}
	location, err := locateHostProtection(directory)
	if err != nil || location != *p.lease.record.Journal {
		return ErrHostProtectionChanged
	}
	identity, err := protectionJournalIdentity(directory)
	if err != nil || identity != p.journalIdentity {
		return ErrHostProtectionChanged
	}
	return p.lease.releaseVerified(ctx)
}

// RetryProtectionSettlement only retries publication by this original surviving
// endpoint after its exact native owner confirmed cleanup. It cannot reconstruct
// an owner from a row, accept a cleanup boolean, start work or reopen a journal.
func (e *StdioEndpoint) RetryProtectionSettlement(ctx context.Context) error {
	if e == nil {
		return ErrAuthority
	}
	e.protectionMu.Lock()
	owner, proof := e.protection, e.protectionCleanup
	e.protectionMu.Unlock()
	if owner == nil {
		return ErrUnsupported
	}
	if proof == nil {
		return ErrHostProtectionUncertain
	}
	return owner.settle(ctx)
}

// Constructed solely from the retained native owner's successful release.
type stdioNativeCleanup struct{}

func (e *StdioEndpoint) recordProtectionCleanup(releaseErr error) {
	e.protectionMu.Lock()
	defer e.protectionMu.Unlock()
	if nativeCleanupVerified(releaseErr) {
		e.protectionCleanup = &stdioNativeCleanup{}
	}
}

func (e *StdioEndpoint) finishProtection(ctx context.Context) error {
	err := e.RetryProtectionSettlement(ctx)
	if errors.Is(err, ErrUnsupported) {
		return nil
	}
	return err
}
