package models

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// GGUFBundle is data-only, pinned before transfer. A nil projector means a
// single-file model, not a claim that the model supports image input.
type GGUFBundle struct {
	SchemaVersion int              `json:"schema_version"`
	Repository    string           `json:"repository"`
	Revision      string           `json:"revision"`
	Weights       PackageArtifact  `json:"weights"`
	Projector     *PackageArtifact `json:"projector,omitempty"`
}

// GGUF downloads historically support large language models. Do not apply the
// smaller speech-package limit to this compatibility path.
const maxGGUFBundleBytes int64 = 1 << 40

func (b GGUFBundle) Validate() error {
	if b.SchemaVersion != 1 || !ValidHubRepository(b.Repository) || !sourceRevision.MatchString(b.Revision) {
		return acquisitionError("invalid_source", "Invalid model bundle identity.")
	}
	for i, a := range b.Artifacts() {
		role := "weights"
		if i == 1 {
			role = "projector"
		}
		if a.Role != role || !validArtifactPath(a.Path) || !strings.HasSuffix(strings.ToLower(a.Path), ".gguf") || a.Size <= 0 || a.Size > maxGGUFBundleBytes || !packageDigest.MatchString(a.SHA256) || a.Source == nil || a.Source.Repository != "https://huggingface.co/"+b.Repository || a.Source.Revision != b.Revision || a.Source.Path != a.Path {
			return acquisitionError("invalid_source", "Invalid model bundle artifact.")
		}
	}
	if b.Projector != nil && b.Projector.Path == b.Weights.Path {
		return acquisitionError("invalid_source", "Weights and projector must be distinct artifacts.")
	}
	if b.TotalBytes() > maxGGUFBundleBytes {
		return acquisitionError("package_too_large", "Model bundle exceeds the supported size limit.")
	}
	return nil
}

func (b GGUFBundle) Artifacts() []PackageArtifact {
	a := []PackageArtifact{b.Weights}
	if b.Projector != nil {
		a = append(a, *b.Projector)
	}
	return a
}

func (b GGUFBundle) TotalBytes() int64 {
	total := b.Weights.Size
	if b.Projector != nil {
		total += b.Projector.Size
	}
	return total
}

var bundleQuantSuffix = regexp.MustCompile(`(?i)[._-](?:ud-)?(?:iq[0-9][a-z0-9_]*|q[0-9][a-z0-9_]*|bf16|f16|f32)$`)

func bundleFamily(filename string) string {
	stem := strings.ToLower(strings.TrimSuffix(path.Base(filename), path.Ext(filename)))
	stem = bundleQuantSuffix.ReplaceAllString(stem, "")
	stem = strings.TrimPrefix(stem, "mmproj-")
	stem = strings.TrimPrefix(stem, "projector-")
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, "-mmproj"), ".mmproj")
	return stem
}

func auxiliaryGGUF(filename string) bool {
	n := strings.ToLower(path.Base(filename))
	return strings.HasPrefix(n, "dflash-") || strings.HasPrefix(n, "imatrix") || strings.HasPrefix(n, "draft-") || strings.HasPrefix(n, "mtp-")
}

func selectBundleProjector(files []hubPackageFile, selected string) (*hubPackageFile, error) {
	family := bundleFamily(selected)
	families := map[string]bool{}
	var projectors, candidates []hubPackageFile
	for _, f := range files {
		if !strings.EqualFold(path.Ext(f.Path), ".gguf") || auxiliaryGGUF(f.Path) || splitGGUF.MatchString(f.Path) {
			continue
		}
		if isProjectorFilename(f.Path) {
			projectors = append(projectors, f)
		} else {
			families[bundleFamily(f.Path)] = true
		}
	}
	for _, f := range projectors {
		pf := bundleFamily(f.Path)
		generic := pf == "mmproj" || pf == "projector"
		if pf == family || generic && len(families) == 1 && families[family] && path.Dir(f.Path) == path.Dir(selected) {
			candidates = append(candidates, f)
		}
	}
	if len(projectors) == 0 {
		return nil, nil
	}
	if len(candidates) == 0 {
		return nil, acquisitionError("projector_ambiguous", "This repository contains vision files, but their relationship to the selected model is ambiguous. Choose a model with a complete matching companion layout.")
	}
	// Precision alternatives for the same family are not separate dependencies.
	rank := func(f hubPackageFile) int {
		n := strings.ToLower(f.Path)
		if strings.HasSuffix(n, "-f16.gguf") || strings.HasSuffix(n, ".f16.gguf") {
			return 0
		}
		if strings.HasSuffix(n, "-bf16.gguf") || strings.HasSuffix(n, ".bf16.gguf") {
			return 1
		}
		if strings.HasSuffix(n, "-q8_0.gguf") || strings.HasSuffix(n, ".q8_0.gguf") {
			return 2
		}
		return 3
	}
	sort.Slice(candidates, func(i, j int) bool {
		if rank(candidates[i]) == rank(candidates[j]) {
			return candidates[i].Path < candidates[j].Path
		}
		return rank(candidates[i]) < rank(candidates[j])
	})
	if len(candidates) > 1 && rank(candidates[0]) == rank(candidates[1]) {
		return nil, acquisitionError("projector_ambiguous", "More than one matching vision projector was found. An explicit model recipe is required.")
	}
	return &candidates[0], nil
}

func (hf *HuggingFaceClient) ResolveGGUFBundle(ctx context.Context, repo, revision, filename string) (*GGUFBundle, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !validArtifactPath(filename) || standaloneFileReason(filename) != "" || auxiliaryGGUF(filename) {
		return nil, acquisitionError("unsupported_layout", "Select complete language-model weights, not a projector, draft model, importance matrix or split shard.")
	}
	meta, files, err := hf.packageSnapshot(ctx, repo, revision)
	if err != nil {
		return nil, err
	}
	var weights *hubPackageFile
	for i := range files {
		if files[i].Path == filename {
			weights = &files[i]
			break
		}
	}
	if weights == nil {
		return nil, acquisitionError("source_missing", "Selected model weights are not present in the resolved revision.")
	}
	projector, err := selectBundleProjector(files, filename)
	if err != nil {
		return nil, err
	}
	if projector == nil && (findProjectorFallback(repo, filename) != nil || meta.PipelineTag == "image-text-to-text" || meta.PipelineTag == "visual-question-answering") {
		return nil, acquisitionError("projector_missing", "This vision repository does not contain a matching projector. Choose a complete model repository; OffGrid will not substitute a projector from another source.")
	}
	b := &GGUFBundle{SchemaVersion: 1, Repository: repo, Revision: meta.SHA}
	artifact := func(f hubPackageFile, role string) (PackageArtifact, error) {
		if f.LFS == nil || f.LFS.Size != f.Size || !packageDigest.MatchString(f.LFS.OID) {
			return PackageArtifact{}, acquisitionError("source_identity_missing", fmt.Sprintf("The %s artifact has no verified download digest.", role))
		}
		return PackageArtifact{Path: f.Path, Role: role, Size: f.Size, SHA256: f.LFS.OID, License: meta.Card.License, Source: &ArtifactSource{Repository: "https://huggingface.co/" + repo, Revision: meta.SHA, Path: f.Path}}, nil
	}
	b.Weights, err = artifact(*weights, "weights")
	if err != nil {
		return nil, err
	}
	if projector != nil {
		a, err := artifact(*projector, "projector")
		if err != nil {
			return nil, err
		}
		b.Projector = &a
	}
	return b, b.Validate()
}
