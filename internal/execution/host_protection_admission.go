package execution

import (
	"context"
	"path/filepath"
	"strings"
)

func protectionBindingValid(binding Binding) bool {
	return binding.Revision > 0 && validIdentity(binding.Actor) && validIdentity(binding.Workspace) && validIdentity(binding.Host) && validIdentity(binding.Session) && validIdentity(binding.Grant) && validIdentity(binding.Job)
}

// RegisterBrowserRoot records a trusted supervisor's chosen private location.
// Reservations survive checker/browser exit and cover missing descendants.
func (s *HostProtectionRegistry) RegisterBrowserRoot(ctx context.Context, directory string) (*ProtectedBrowserRoot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	location, err := s.validateLocation(directory, false)
	if err != nil {
		return nil, err
	}
	if err = s.checkAncestry(directory); err != nil {
		return nil, err
	}
	if location.Suffix == "" && CheckPrivatePath(location.Path) != nil {
		return nil, ErrAuthority
	}
	record := hostProtectionRecord{Version: 1, ID: protectionPathKey(location.Path), Kind: "browser", State: "protected", Root: location}
	capability := ""
	if location.Suffix != "" {
		capability, err = opaqueToken()
		if err != nil {
			return nil, err
		}
		record.State = "pending"
		record.CapabilityHash = protectionHash(capability)
	}
	var result *ProtectedBrowserRoot
	err = s.transaction(ctx, func(store *hostProtectionTransaction) error {
		current, err := locateHostProtection(directory)
		if err != nil || current != location {
			return ErrHostProtectionChanged
		}
		if existing, ok := store.records[record.ID]; ok && existing.Kind == "browser" && existing.State == "protected" && existing.Root != location {
			return ErrHostProtectionChanged
		}
		if err = store.checkLiveLocations(ctx); err != nil {
			return err
		}
		for _, existing := range store.records {
			if existing.Kind == "execution" && existing.State != "released" && (protectionOverlap(existing.Root.Path, location.Path) || protectionOverlap(existing.Journal.Path, location.Path)) {
				return ErrHostProtectionConflict
			}
		}
		if existing, ok := store.records[record.ID]; ok {
			if existing.Kind != "browser" {
				return ErrHostProtectionConflict
			}
			if existing.State == "pending" {
				return ErrHostProtectionUncertain
			}
			if existing.Root != location {
				return ErrHostProtectionChanged
			}
			result = &ProtectedBrowserRoot{registry: s, record: existing}
			return nil
		}
		if err = store.reclaimReleased(ctx); err != nil {
			return err
		}
		if err = store.insert(ctx, record); err != nil {
			return err
		}
		result = &ProtectedBrowserRoot{registry: s, record: record, capability: capability}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReserveExecution must commit before worker activation. It records process
// authority, but disjoint roots alone do NOT contain unrestricted processes.
// Runtime policy must independently gate those processes or use verified OS
// isolation. This primitive must not be advertised as that containment gate.
func (s *HostProtectionRegistry) ReserveExecution(ctx context.Context, config Context, journalDirectory string) (*HostProtectionLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if config.Version != ProtocolVersion || !protectionBindingValid(config.Binding) || !validBackend(config) {
		return nil, ErrAuthority
	}
	directory := config.Directory
	if directory == "" {
		directory = "."
	}
	clean := filepath.Clean(directory)
	if filepath.IsAbs(directory) || strings.ContainsRune(directory, 0) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, ErrPath
	}
	root, err := s.validateLocation(config.Root, true)
	if err != nil {
		return nil, err
	}
	journal, err := s.validateLocation(journalDirectory, true)
	if err != nil {
		return nil, err
	}
	if err = s.checkAncestry(journalDirectory); err != nil {
		return nil, err
	}
	if protectionOverlap(root.Path, journal.Path) || CheckPrivatePath(journal.Path) != nil {
		return nil, ErrAuthority
	}
	id, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	capability, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	binding := config.Binding
	record := hostProtectionRecord{Version: 1, ID: id, Kind: "execution", State: "reserved", Root: root, Journal: &journal, Binding: &binding, Backend: config.Backend, Permissions: config.Permissions, CapabilityHash: protectionHash(capability)}
	err = s.transaction(ctx, func(store *hostProtectionTransaction) error {
		if err := store.checkLiveLocations(ctx); err != nil {
			return err
		}
		current, err := locateHostProtection(config.Root)
		if err != nil || current != root {
			return ErrHostProtectionChanged
		}
		current, err = locateHostProtection(journalDirectory)
		if err != nil || current != journal {
			return ErrHostProtectionChanged
		}
		for _, existing := range store.records {
			if existing.Kind == "browser" && (protectionOverlap(existing.Root.Path, root.Path) || protectionOverlap(existing.Root.Path, journal.Path)) {
				return ErrHostProtectionConflict
			}
			// Only a verified release permits an explicit matching restart. It
			// receives a fresh capability; the existing journal owns no-replay.
			if existing.Kind == "execution" && existing.Binding.Session == binding.Session {
				if existing.State != "released" {
					return ErrHostProtectionUncertain
				}
				if *existing.Binding != binding || existing.Root != root || *existing.Journal != journal || existing.Backend != config.Backend || existing.Permissions != config.Permissions {
					return ErrHostProtectionChanged
				}
			}
		}
		if err := store.reclaimReleased(ctx); err != nil {
			return err
		}
		return store.insert(ctx, record)
	})
	if err != nil {
		return nil, err
	}
	return &HostProtectionLease{registry: s, id: id, capability: capability, record: record, configHash: protectionConfigHash(config)}, nil
}
