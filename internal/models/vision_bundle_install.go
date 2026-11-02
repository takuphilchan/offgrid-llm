package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

const modelAssetsDirectory = ".model-assets"

type modelAssetIdentity struct {
	Size     int64 `json:"size"`
	Modified int64 `json:"modified_ns"`
}

type modelCompanionReceipt struct {
	SchemaVersion     int                `json:"schema_version"`
	Bundle            GGUFBundle         `json:"bundle"`
	ModelFile         string             `json:"model_file"`
	ProjectorFile     string             `json:"projector_file"`
	WeightsIdentity   modelAssetIdentity `json:"weights_identity"`
	ProjectorIdentity modelAssetIdentity `json:"projector_identity"`
}

func companionPath(local string, b GGUFBundle) string {
	key := sha256.Sum256([]byte(local))
	return fmt.Sprintf("%s/%x/%s.gguf", modelAssetsDirectory, key[:12], b.Projector.SHA256)
}

func assetIdentity(root *os.Root, name string) (modelAssetIdentity, error) {
	f, err := regularPackageFile(root, name)
	if err != nil {
		return modelAssetIdentity{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return modelAssetIdentity{}, err
	}
	return modelAssetIdentity{Size: info.Size(), Modified: info.ModTime().UnixNano()}, nil
}

func verifyBundleFile(ctx context.Context, root *os.Root, name string, a PackageArtifact) (modelAssetIdentity, error) {
	before, err := assetIdentity(root, name)
	if err != nil {
		return before, err
	}
	f, err := regularPackageFile(root, name)
	if err != nil {
		return before, err
	}
	err = hashPackageFile(ctx, f, a.Size, a.SHA256)
	f.Close()
	if err != nil {
		return before, err
	}
	after, err := assetIdentity(root, name)
	if err != nil || before != after {
		return before, acquisitionError("artifact_changed", "A model artifact changed during verification.")
	}
	return before, nil
}

// AcquireGGUFBundle is invoked only by an authorized download operation. It does
// not overwrite an existing model, substitute revisions, or mark vision ready
// before both artifacts have passed independent digest verification.
func (hf *HuggingFaceClient) AcquireGGUFBundle(ctx context.Context, dir, local string, b GGUFBundle, progress func(int64, int64)) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if !validArtifactPath(local) || path.Base(local) != local || filepath.Base(local) != local {
		return acquisitionError("invalid_target", "Invalid local model filename.")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var complete int64
	install := func(target string, a PackageArtifact) (modelAssetIdentity, error) {
		if _, err := root.Lstat(target); errors.Is(err, fs.ErrNotExist) {
			partial := target + "." + a.SHA256[:16] + ".tmp"
			if err := hf.transferPackageArtifact(ctx, root, partial, a, func(done int64) error {
				if progress != nil {
					progress(complete+done, b.TotalBytes())
				}
				return ctx.Err()
			}); err != nil {
				return modelAssetIdentity{}, err
			}
			if err := ctx.Err(); err != nil {
				return modelAssetIdentity{}, err
			}
			// Exclusive publication: a concurrently created destination is never
			// replaced. The verified temporary file remains recoverable on failure.
			if err := root.Link(partial, target); err != nil {
				return modelAssetIdentity{}, err
			}
			if err := root.Remove(partial); err != nil {
				return modelAssetIdentity{}, err
			}
		} else if err != nil {
			return modelAssetIdentity{}, err
		}
		identity, err := verifyBundleFile(ctx, root, target, a)
		if err != nil {
			return identity, err
		}
		complete += a.Size
		if progress != nil {
			progress(complete, b.TotalBytes())
		}
		return identity, nil
	}
	weights, err := install(local, b.Weights)
	if err != nil {
		return err
	}
	if b.Projector == nil {
		return nil
	}
	target := companionPath(local, b)
	if err := root.MkdirAll(path.Dir(target), 0700); err != nil {
		return err
	}
	if err := checkedDirectory(root, path.Dir(target)); err != nil {
		return err
	}
	// Repair only a digest-addressed companion owned by this model. Never
	// replace user weights, follow links, or discard the damaged original.
	if _, err := assetIdentity(root, target); err == nil {
		if _, err := verifyBundleFile(ctx, root, target, *b.Projector); err != nil {
			var failure *ModelAcquisitionError
			if !errors.As(err, &failure) || failure.Code != "artifact_integrity_failed" {
				return err
			}
			if err := root.Rename(target, target+".corrupt-"+rand.Text()); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	projector, err := install(target, *b.Projector)
	if err != nil {
		return err
	}
	if current, err := assetIdentity(root, local); err != nil || current != weights {
		return acquisitionError("artifact_changed", "Model weights changed while installing the vision companion.")
	}
	receipt := modelCompanionReceipt{SchemaVersion: 1, Bundle: b, ModelFile: local, ProjectorFile: target, WeightsIdentity: weights, ProjectorIdentity: projector}
	temp := local + ".assets-" + rand.Text() + ".tmp"
	if err := writePackageJSON(root, temp, receipt); err != nil {
		return err
	}
	defer root.Remove(temp)
	if err := ctx.Err(); err != nil {
		return err
	}
	return root.Rename(temp, local+".assets.json")
}

// InstalledCompanion reads a receipt written only after hashing both files.
// Changed file identity fails closed; explicit repair rehashes before rebinding.
// This is an integrity cache for local model data, not a signed trust grant.
func InstalledCompanion(modelPath string) (string, bool) {
	root, err := os.OpenRoot(filepath.Dir(modelPath))
	if err != nil {
		return "", false
	}
	defer root.Close()
	local := filepath.Base(modelPath)
	var receipt modelCompanionReceipt
	if readPackageJSON(root, local+".assets.json", &receipt) != nil || receipt.SchemaVersion != 1 || receipt.ModelFile != local || receipt.Bundle.Validate() != nil || receipt.Bundle.Projector == nil || receipt.ProjectorFile != companionPath(local, receipt.Bundle) {
		return "", false
	}
	if checkedDirectory(root, path.Dir(receipt.ProjectorFile)) != nil {
		return "", false
	}
	weights, err := assetIdentity(root, local)
	if err != nil || weights != receipt.WeightsIdentity || weights.Size != receipt.Bundle.Weights.Size {
		return "", false
	}
	projector, err := assetIdentity(root, receipt.ProjectorFile)
	if err != nil || projector != receipt.ProjectorIdentity || projector.Size != receipt.Bundle.Projector.Size {
		return "", false
	}
	return filepath.Join(filepath.Dir(modelPath), filepath.FromSlash(receipt.ProjectorFile)), true
}
