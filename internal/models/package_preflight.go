package models

import (
	"fmt"
	"net/url"
	"strings"
)

type PackagePreflight struct {
	TransferBytes     int64          `json:"transfer_bytes"`
	RequiredFreeBytes int64          `json:"required_free_bytes"`
	AvailableBytes    int64          `json:"available_bytes"`
	Runtime           PackageRuntime `json:"runtime"`
	RuntimeAvailable  bool           `json:"runtime_available"`
	Readiness         ModelReadiness `json:"readiness"`
	Warnings          []string       `json:"warnings"`
}

// ValidateAcquisitionManifest is stricter than local import. Source declarations
// on a locally supplied manifest are not sufficient to establish provenance;
// callers must also bind it to a service-generated resolution or catalog receipt.
func ValidateAcquisitionManifest(m ModelPackageManifest) error {
	if err := m.Validate(); err != nil {
		return acquisitionError("invalid_manifest", err.Error())
	}
	for _, a := range m.Artifacts {
		if a.Source == nil {
			return acquisitionError("source_required", "Re-import this local package; no verified repair source is recorded.")
		}
		u, err := url.Parse(a.Source.Repository)
		if err != nil || !validHubURL(u, true) || u.RawQuery != "" || u.Host != "huggingface.co" || !ValidHubRepository(strings.TrimPrefix(u.Path, "/")) || !sourceRevision.MatchString(a.Source.Revision) || !validArtifactPath(a.Source.Path) {
			return acquisitionError("unsafe_source", "Package source is not an immutable public Hugging Face artifact.")
		}
	}
	return nil
}

func PreflightPackage(m ModelPackageManifest, available, previousBytes int64) (PackagePreflight, error) {
	var p PackagePreflight
	if err := ValidateAcquisitionManifest(m); err != nil {
		return p, err
	}
	if previousBytes < 0 || previousBytes > MaxPackageBytes || available < 0 {
		return p, acquisitionError("space_unavailable", "Could not determine safe package storage requirements.")
	}
	for _, a := range m.Artifacts {
		p.TransferBytes += a.Size
	}
	// Reserve staging plus publication/recovery copy, keeping a prior revision
	// intact. The transfer manager may use less, but never advertises disk it
	// cannot reserve safely. Existing partial bytes are not trusted savings.
	p.RequiredFreeBytes = p.TransferBytes*2 + previousBytes + (16 << 20)
	p.AvailableBytes = available
	p.Runtime = m.Runtime
	p.Readiness = ModelReadiness{Integrity: "unchecked", Issue: "runtime_not_checked"}
	p.Warnings = []string{"Runtime and model compatibility must be checked separately. Downloading weights does not establish working speech or interactive latency.", "Review model and dependency licenses before downloading."}
	if available < p.RequiredFreeBytes {
		return p, acquisitionError("insufficient_space", fmt.Sprintf("Package requires %d free bytes including staging and recovery; %d are available.", p.RequiredFreeBytes, available))
	}
	return p, nil
}

func (s *PackageStore) Preflight(m ModelPackageManifest, previousBytes int64) (PackagePreflight, error) {
	root, err := s.open()
	if err != nil {
		return PackagePreflight{}, acquisitionError("space_unavailable", "Model storage is not accessible.")
	}
	root.Close()
	available, err := getDiskSpace(s.modelsDir)
	if err != nil {
		return PackagePreflight{}, acquisitionError("space_unavailable", "Cannot inspect free model storage space.")
	}
	return PreflightPackage(m, available, previousBytes)
}
