package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpeechExecutableBesideService(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "bin", "offgrid.exe")
	for _, adapter := range []string{"whisper", "piper"} {
		t.Run(adapter, func(t *testing.T) {
			dir := filepath.Join(root, "speech-runtime", adapter)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if got := speechExecutableBeside(executable, adapter, []string{"worker.exe"}); got != "" {
				t.Fatal(got)
			}
			// A directory with an executable name is not a runtime.
			if err := os.Mkdir(filepath.Join(dir, "directory.exe"), 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "worker.exe")
			if err := os.WriteFile(file, []byte("fixture"), 0700); err != nil {
				t.Fatal(err)
			}
			if got := speechExecutableBeside(executable, adapter, []string{"directory.exe", "worker.exe"}); got != file {
				t.Fatalf("got %q want %q", got, file)
			}
			if got := speechExecutableBeside(filepath.Join(root, "other", "bin", "offgrid.exe"), adapter, []string{"worker.exe"}); got != "" {
				t.Fatalf("cross-install discovery: %q", got)
			}
		})
	}
}

func TestSpeechRuntimeStatusDoesNotRequireLegacyModelFiles(t *testing.T) {
	engine, err := NewEngine(Config{DataDir: t.TempDir(), WhisperPath: "configured-whisper", PiperPath: "configured-piper"})
	if err != nil {
		t.Fatal(err)
	}
	status := engine.Status()
	for _, kind := range []string{"asr", "tts"} {
		entry := status[kind].(map[string]interface{})
		if entry["runtime"] != true || entry["available"] != false {
			t.Fatalf("runtime/model distinction lost: %+v", entry)
		}
	}
}

func TestCLISpeechPayloadBesideService(t *testing.T) {
	root := t.TempDir()
	for _, adapter := range []string{"whisper", "piper"} {
		dir := filepath.Join(root, "audio", adapter)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "worker.exe")
		if err := os.WriteFile(file, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
		if got := speechExecutableBeside(filepath.Join(root, "offgrid.exe"), adapter, []string{"worker.exe"}); got != file {
			t.Fatalf("CLI payload not found: %q", got)
		}
		if got := speechExecutableBeside(filepath.Join(root, "another", "offgrid.exe"), adapter, []string{"worker.exe"}); got != "" {
			t.Fatalf("cross-install payload found: %q", got)
		}
	}
}
