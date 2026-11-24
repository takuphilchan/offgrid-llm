package execution

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
)

func applyIntegrationStep(ctx context.Context, target, private *os.Root, step integrationStep, limit int64) (receipt IntegrationFileReceipt, resultErr error) {
	change := step.Change
	receipt = IntegrationFileReceipt{Ordinal: step.Ordinal, Path: change.Path, Operation: change.Kind, State: "uncertain"}
	defer func() {
		if resultErr == nil {
			receipt.State = "integrated"
			return
		}
		receipt.Error = errorCode(resultErr)
		if errors.Is(resultErr, ErrConflict) || errors.Is(resultErr, ErrRevision) || errors.Is(resultErr, os.ErrExist) {
			receipt.State = "conflict"
		}
	}()
	if change.Kind == "update" && change.Base != nil && change.Result != nil && change.Base.Kind == "file" && change.Result.Kind == "file" {
		actual, err := replacePreserving(ctx, target, change.Path, private, change.Result.SHA256, "displaced", change.Base.SHA256, change.Result.SHA256, limit)
		receipt.Dispatched, receipt.Displaced, receipt.DisplacedDigest, receipt.ResultDigest = actual.Dispatched, actual.Displaced, actual.DisplacedDigest, actual.ResultDigest
		return receipt, err
	}
	to, err := pinReplacementDirectory(target, path.Dir(change.Path))
	if err != nil {
		return receipt, err
	}
	defer to.close()
	from, err := pinReplacementDirectory(private, ".")
	if err != nil {
		return receipt, err
	}
	defer from.close()
	if err = qualifyReplacementVolume(to, from); err != nil {
		return receipt, err
	}
	targetView, err := replacementView(target, path.Dir(change.Path), to)
	if err != nil {
		return receipt, err
	}
	defer targetView.Close()
	privateView, err := replacementView(private, ".", from)
	if err != nil {
		return receipt, err
	}
	defer privateView.Close()
	name := path.Base(change.Path)
	switch change.Kind {
	case "create":
		if change.Base != nil || change.Result == nil {
			return receipt, ErrRevision
		}
		if _, err = targetView.Lstat(name); !os.IsNotExist(err) {
			if err == nil {
				err = ErrConflict
			}
			return receipt, err
		}
		staged := "new-directory"
		if change.Result.Kind == "file" {
			staged = change.Result.SHA256
		}
		if err = ctx.Err(); err != nil {
			return receipt, err
		}
		receipt.Dispatched = true
		if err = movePinnedExclusive(from, staged, to, name); err != nil {
			return receipt, &MutationError{Cause: err, Effects: "unknown"}
		}
		if change.Result.Kind == "file" {
			var resultInfo os.FileInfo
			receipt.ResultDigest, resultInfo, err = observeReplacementFile(context.WithoutCancel(ctx), targetView, name, limit)
			if err != nil {
				return receipt, err
			}
			if receipt.ResultDigest != change.Result.SHA256 {
				return receipt, ErrConflict
			}
			if err = syncReplacementFile(targetView, name, resultInfo); err != nil {
				return receipt, err
			}
		} else if err = verifyEmptyIntegrationDirectory(targetView, name); err != nil {
			return receipt, err
		}
	case "delete":
		if change.Base == nil || change.Result != nil {
			return receipt, ErrRevision
		}
		if change.Base.Kind == "file" {
			before, err := replacementFileDigest(ctx, targetView, name, limit)
			if err != nil {
				return receipt, err
			}
			if before != change.Base.SHA256 {
				return receipt, ErrConflict
			}
		} else if err = verifyEmptyIntegrationDirectory(targetView, name); err != nil {
			return receipt, err
		}
		if err = ctx.Err(); err != nil {
			return receipt, err
		}
		receipt.Dispatched, receipt.Displaced = true, "displaced"
		if err = movePinnedExclusive(to, name, from, "displaced"); err != nil {
			return receipt, &MutationError{Cause: err, Effects: "unknown"}
		}
		if change.Base.Kind == "file" {
			var displacedInfo os.FileInfo
			receipt.DisplacedDigest, displacedInfo, err = observeReplacementFile(context.WithoutCancel(ctx), privateView, "displaced", limit)
			if err != nil {
				return receipt, err
			}
			if receipt.DisplacedDigest != change.Base.SHA256 {
				return receipt, ErrConflict
			}
			if err = syncReplacementFile(privateView, "displaced", displacedInfo); err != nil {
				return receipt, err
			}
		} else if err = verifyEmptyIntegrationDirectory(privateView, "displaced"); err != nil {
			return receipt, err
		}
		if _, err = targetView.Lstat(name); !os.IsNotExist(err) {
			if err == nil {
				err = ErrConflict
			}
			return receipt, err
		}
	default:
		return receipt, ErrUnsupported
	}
	currentView, err := replacementView(target, path.Dir(change.Path), to)
	if err != nil {
		return receipt, errors.Join(ErrConflict, err)
	}
	currentView.Close()
	if err = syncReplacement(to, from); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func verifyEmptyIntegrationDirectory(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrConflict
	}
	directory, err := root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer directory.Close()
	current, err := directory.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return ErrConflict
	}
	f, err := directory.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.ReadDir(1)
	if len(entries) > 0 {
		return ErrConflict
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
