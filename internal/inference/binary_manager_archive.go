package inference

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxLlamaExtractedBytes int64 = 1 << 30

func extractLlamaArchive(archivePath, stage, version, asset string) error {
	if strings.HasSuffix(asset, ".tar.gz") {
		return extractLlamaTarGz(archivePath, stage, version)
	}
	if strings.HasSuffix(asset, ".zip") {
		return extractLlamaZip(archivePath, stage, version)
	}
	return fmt.Errorf("unsupported llama.cpp archive format: %s", asset)
}

func llamaArchivePath(stage, version, entry string) (string, error) {
	entry = strings.ReplaceAll(entry, "\\", "/")
	if strings.HasPrefix(entry, "/") || strings.Contains(entry, ":") {
		return "", fmt.Errorf("unsafe archive path %q", entry)
	}
	for _, component := range strings.Split(entry, "/") {
		if component == ".." {
			return "", fmt.Errorf("unsafe archive path %q", entry)
		}
	}
	clean := path.Clean(entry)
	if clean == "." || clean == "llama-"+version {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe archive path %q", entry)
	}
	if strings.HasPrefix(clean, "llama-"+version+"/") {
		clean = strings.TrimPrefix(clean, "llama-"+version+"/")
	} else if strings.Contains(clean, "/") {
		return "", fmt.Errorf("unexpected archive root %q", entry)
	}
	destination := filepath.Join(stage, filepath.FromSlash(clean))
	if !insideLlamaStage(stage, destination) {
		return "", fmt.Errorf("unsafe archive path %q", entry)
	}
	return destination, nil
}

func insideLlamaStage(stage, destination string) bool {
	relative, err := filepath.Rel(stage, destination)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func writeLlamaArchiveFile(destination string, source io.Reader, size int64, mode os.FileMode) error {
	if size < 0 || size > maxLlamaExtractedBytes {
		return fmt.Errorf("archive member exceeds extraction limit")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(file, source, size)
	closeErr := file.Close()
	if copyErr != nil || written != size {
		return fmt.Errorf("extract %s: %w", destination, copyErr)
	}
	return closeErr
}

func extractLlamaTarGz(archivePath, stage, version string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var extracted int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		destination, err := llamaArchivePath(stage, version, header.Name)
		if err != nil {
			return err
		}
		if destination == "" {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			extracted += header.Size
			if extracted > maxLlamaExtractedBytes {
				return fmt.Errorf("llama.cpp archive exceeds extraction limit")
			}
			if err := writeLlamaArchiveFile(destination, reader, header.Size, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if path.IsAbs(header.Linkname) || strings.Contains(header.Linkname, ":") {
				return fmt.Errorf("unsafe archive symlink %q", header.Name)
			}
			if !insideLlamaStage(stage, filepath.Clean(filepath.Join(filepath.Dir(destination), filepath.FromSlash(header.Linkname)))) {
				return fmt.Errorf("archive symlink escapes extraction directory: %q", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, destination); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive member type %d at %q", header.Typeflag, header.Name)
		}
	}
}

func extractLlamaZip(archivePath, stage, version string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	var extracted int64
	for _, member := range reader.File {
		destination, err := llamaArchivePath(stage, version, member.Name)
		if err != nil {
			return err
		}
		if destination == "" {
			continue
		}
		if member.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			continue
		}
		if !member.Mode().IsRegular() {
			return fmt.Errorf("unsupported zip member %q", member.Name)
		}
		extracted += int64(member.UncompressedSize64)
		if extracted > maxLlamaExtractedBytes {
			return fmt.Errorf("llama.cpp archive exceeds extraction limit")
		}
		input, err := member.Open()
		if err != nil {
			return err
		}
		mode := member.Mode()
		if mode.Perm() == 0 {
			mode = 0o755
		}
		writeErr := writeLlamaArchiveFile(destination, input, int64(member.UncompressedSize64), mode)
		closeErr := input.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
