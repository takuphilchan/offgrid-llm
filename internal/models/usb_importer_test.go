package models

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUSBImportSpaceAdmissionOnlyIncludesManifestModels(t *testing.T) {
	for _, enough := range []bool{false, true} {
		t.Run(fmt.Sprint(enough), func(t *testing.T) {
			usb, dest := t.TempDir(), t.TempDir()
			data := importGGUFFixture(t, false)
			digest := sha256.Sum256(data)
			for _, name := range []string{"selected.gguf", "unlisted.gguf"} {
				if err := os.WriteFile(filepath.Join(usb, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			manifest := fmt.Sprintf(`{"models":[{"file_name":"selected.gguf","sha256":"%x"}]}`, digest)
			if err := os.WriteFile(filepath.Join(usb, "manifest.json"), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			available := int64(len(data))
			if !enough {
				available--
			}
			importer := NewUSBImporter(dest, NewRegistry(dest))
			count, err := importer.importAll(usb, nil, func(string) (int64, error) { return available, nil })
			if enough {
				if err != nil || count != 1 {
					t.Fatalf("space for the selected model must suffice: count=%d err=%v", count, err)
				}
				got, err := os.ReadFile(filepath.Join(dest, "selected.gguf"))
				if err != nil || !bytes.Equal(got, data) {
					t.Fatalf("selected bytes not imported intact: %v", err)
				}
			} else if err == nil || count != 0 || !strings.Contains(err.Error(), "insufficient disk space") {
				t.Fatalf("insufficient selected-model capacity accepted: count=%d err=%v", count, err)
			}
			if _, err := os.Stat(filepath.Join(dest, "unlisted.gguf")); !os.IsNotExist(err) {
				t.Fatal("unlisted model imported")
			}
		})
	}
}

func TestUSBImportPreservesExistingModelOnConflict(t *testing.T) {
	usb, dest := t.TempDir(), t.TempDir()
	old := importGGUFFixture(t, false)
	incoming := append([]byte(nil), old...)
	incoming[len(incoming)-1] = 1
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dest, "model.gguf"), old)
	write(filepath.Join(usb, "model.gguf"), incoming)
	importer := NewUSBImporter(dest, NewRegistry(dest))
	if err := importer.ImportModel(filepath.Join(usb, "model.gguf"), "", nil); err == nil {
		t.Fatal("conflicting model was replaced")
	}
	got, _ := os.ReadFile(filepath.Join(dest, "model.gguf"))
	if !bytes.Equal(got, old) {
		t.Fatal("existing model changed")
	}
	write(filepath.Join(usb, "model.gguf"), old)
	count, err := importer.ImportAll(usb, nil)
	if err != nil || count != 0 {
		t.Fatalf("identical retry: %d %v", count, err)
	}
}

func TestRegistryUSBImportDoesNotDeadlockOrEscape(t *testing.T) {
	usb, dest := t.TempDir(), t.TempDir()
	source := filepath.Join(usb, "model.gguf")
	if err := os.WriteFile(source, importGGUFFixture(t, false), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(dest)
	done := make(chan error, 1)
	go func() { done <- registry.ImportFromUSB(source, "model.gguf") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("registry import deadlocked")
	}
	if err := registry.ImportFromUSB(source, "../outside.gguf"); err == nil {
		t.Fatal("unsafe destination accepted")
	}
}

func TestUSBImportRejectsCorruptManifestAndReportsFailures(t *testing.T) {
	for _, manifest := range []string{
		`{`, `{"models":[]}`, `{"models":[{"file_name":"model.gguf","sha256":"bad"}]}`,
	} {
		t.Run(manifest, func(t *testing.T) {
			usb, dest := t.TempDir(), t.TempDir()
			os.WriteFile(filepath.Join(usb, "manifest.json"), []byte(manifest), 0600)
			os.WriteFile(filepath.Join(usb, "model.gguf"), importGGUFFixture(t, false), 0600)
			count, err := NewUSBImporter(dest, NewRegistry(dest)).ImportAll(usb, nil)
			if err == nil || count != 0 {
				t.Fatalf("bad manifest accepted: %d %v", count, err)
			}
			entries, _ := os.ReadDir(dest)
			if len(entries) != 0 {
				t.Fatal("bad manifest wrote data")
			}
		})
	}
	usb, dest := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(usb, "invalid.gguf"), []byte("bad"), 0600)
	if _, err := NewUSBImporter(dest, NewRegistry(dest)).ImportAll(usb, nil); err == nil {
		t.Fatal("bulk import hid failure")
	}
}

func TestUSBImportValidatesBeforePublishingAndPreservesOtherStaging(t *testing.T) {
	usb, dest := t.TempDir(), t.TempDir()
	data := importGGUFFixture(t, false)
	source := filepath.Join(usb, "model.gguf")
	os.WriteFile(source, data, 0600)
	stage := filepath.Join(dest, "model.gguf.tmp")
	os.WriteFile(stage, []byte("another download"), 0600)
	importer := NewUSBImporter(dest, NewRegistry(dest))
	if err := importer.ImportModel(source, strings.Repeat("0", 64), nil); err == nil {
		t.Fatal("bad digest accepted")
	}
	if _, err := os.Stat(filepath.Join(dest, "model.gguf")); !os.IsNotExist(err) {
		t.Fatal("invalid model published")
	}
	digest := sha256.Sum256(data)
	if err := importer.ImportModel(source, hex.EncodeToString(digest[:]), nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(stage)
	if string(got) != "another download" {
		t.Fatal("unrelated partial download altered")
	}
}
