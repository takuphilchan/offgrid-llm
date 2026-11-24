package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

// Stream the inventory rather than allocating a caller-sized slice first.
// A small compressed manifest can otherwise describe millions of empty files.
func decodeBackupManifest(reader io.Reader, expectedFiles int) (_ *BackupManifest, resultErr error) {
	defer func() {
		if resultErr != nil && !errors.Is(resultErr, context.Canceled) && !errors.Is(resultErr, context.DeadlineExceeded) {
			// Decoder errors can quote untrusted field names without a bound.
			resultErr = errors.New("invalid, oversized or inconsistent backup manifest")
		}
	}()
	decoder := json.NewDecoder(io.LimitReader(reader, maxManifestBytes+1))
	decoder.DisallowUnknownFields()
	start, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if start != json.Delim('{') {
		return nil, errors.New("manifest object required")
	}
	manifest := &BackupManifest{}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return nil, errors.New("duplicate manifest field")
		}
		seen[key] = true
		switch key {
		case "format":
			err = decoder.Decode(&manifest.Format)
		case "application_version":
			err = decoder.Decode(&manifest.ApplicationVersion)
			if len(manifest.ApplicationVersion) > 256 {
				return nil, errors.New("application version exceeds limit")
			}
		case "created_at":
			err = decoder.Decode(&manifest.CreatedAt)
		case "scope":
			err = decoder.Decode(&manifest.Scope)
		case "bytes":
			err = decoder.Decode(&manifest.Bytes)
		case "files":
			start, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			if start == nil {
				continue
			} // Retain legacy empty/null inventory compatibility.
			if start != json.Delim('[') {
				return nil, errors.New("inventory array required")
			}
			manifest.Files = make([]BackupFile, 0, expectedFiles)
			for decoder.More() {
				if len(manifest.Files) >= expectedFiles || len(manifest.Files) >= maxBackupFiles {
					return nil, errors.New("inventory exceeds archive entry count")
				}
				var item BackupFile
				if err := decoder.Decode(&item); err != nil {
					return nil, err
				}
				if err := portablePath(item.Path); err != nil {
					return nil, err
				}
				manifest.Files = append(manifest.Files, item)
			}
			end, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			if end != json.Delim(']') {
				return nil, errors.New("invalid inventory end")
			}
		default:
			return nil, errors.New("unknown manifest field")
		}
		if err != nil {
			return nil, err
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if end != json.Delim('}') {
		return nil, errors.New("invalid manifest end")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("trailing manifest data")
	}
	return manifest, nil
}
