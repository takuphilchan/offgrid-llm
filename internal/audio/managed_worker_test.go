package audio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		path, err := exec.LookPath(name)
		if err == nil && exec.Command(path, "-c", "import sys").Run() == nil {
			return path
		}
	}
	t.Skip("Python interpreter unavailable")
	return ""
}

func writeFixture(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

// These fixtures exercise the actual embedded worker protocol, NOT speech
// quality. Imports deliberately print Python and native stdout noise.
func fixtureWorker(t *testing.T) (*managedWorker, string) {
	t.Helper()
	python := testPython(t)
	modules := t.TempDir()
	t.Setenv("PYTHONPATH", modules)
	writeFixture(t, modules, "torch.py", "float32='float32'\ndef set_num_threads(n): pass\n")
	writeFixture(t, modules, "qwen_tts.py", `import os,time
print("\nImport banner",flush=True)
os.write(1,b"native banner\n")
class Qwen3TTSModel:
 @classmethod
 def from_pretrained(cls,*a,**kw):
  assert kw["local_files_only"] is True
  return cls()
 def get_supported_speakers(self): return ["ryan"]
 def generate_custom_voice(self,**kw):
  if kw["text"]=="wait": time.sleep(10)
  if kw["text"]=="fail": raise ValueError("PRIVATE TEXT MUST NOT LEAK")
  return [[0,0,0,0]],24000
`)
	writeFixture(t, modules, "qwen_asr.py", `print("asr banner",flush=True)
class Qwen3ASRModel:
 @classmethod
 def from_pretrained(cls,*a,**kw):
  assert kw["local_files_only"] is True
  return cls()
 def transcribe(self,**kw): return [{"text":"hello"}]
`)
	writeFixture(t, modules, "soundfile.py", `import wave
def write(path,waveform,sr,**kw):
 with wave.open(path,"wb") as f:
  f.setnchannels(1); f.setsampwidth(2); f.setframerate(sr); f.writeframes(b"\0"*80)
`)
	dir := t.TempDir()
	writeFixture(t, dir, "config.json", `{"tts_model_type":"custom_voice"}`)
	w := newManagedWorker(RuntimeConfig{PythonPath: python})
	t.Cleanup(w.close)
	return w, dir
}

func TestManagedWorkerSeparatesDiagnosticsAndCleansOutput(t *testing.T) {
	w, dir := fixtureWorker(t)
	for i := 0; i < 2; i++ {
		result, err := w.speak(context.Background(), SpeechRequest{Input: "hello", ResponseFormat: "wav"}, dir)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(result)
		if !bytes.HasPrefix(b, []byte("RIFF")) {
			t.Fatal("not WAV")
		}
	}
	pid := w.proc.cmd.Process.Pid
	workdir := w.proc.dir
	out, err := w.transcribe(context.Background(), TranscriptionRequest{File: strings.NewReader("fixture"), Filename: "test.wav"}, dir)
	if err != nil || out.Text != "hello" {
		t.Fatalf("%+v %v", out, err)
	}
	if w.proc.cmd.Process.Pid == pid {
		t.Fatal("different architecture reused worker")
	}
	if _, err := os.Stat(workdir); !os.IsNotExist(err) {
		t.Fatal("old script directory remains")
	}
}

func TestManagedWorkerCancellationAndSafeError(t *testing.T) {
	w, dir := fixtureWorker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := w.speak(ctx, SpeechRequest{Input: "wait"}, dir)
	if !errors.Is(err, context.DeadlineExceeded) || w.proc != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	_, err = w.speak(context.Background(), SpeechRequest{Input: "fail"}, dir)
	if err == nil || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestManagedWorkerRejectsBaseAndUnsupportedRequests(t *testing.T) {
	w, dir := fixtureWorker(t)
	for _, kind := range []string{"base", "voice_design", "unknown"} {
		writeFixture(t, dir, "config.json", `{"tts_model_type":"`+kind+`"}`)
		if w.available("tts", dir) {
			t.Fatal("unsupported model offered")
		}
		if _, err := w.speak(context.Background(), SpeechRequest{Input: "hello"}, dir); err == nil {
			t.Fatal("unsupported model executed")
		}
	}
	writeFixture(t, dir, "config.json", `{"tts_model_type":"custom_voice"}`)
	for _, req := range []SpeechRequest{{Input: "hello", ResponseFormat: "mp3"}, {Input: "hello", Speed: 2}} {
		if _, err := w.speak(context.Background(), req, dir); err == nil {
			t.Fatal("unsupported request accepted")
		}
	}
	if w.proc != nil {
		t.Fatal("invalid request launched worker")
	}
}

func TestManagedWorkerRejectsMissingModelDirectory(t *testing.T) {
	w := newManagedWorker(RuntimeConfig{PythonPath: "python3"})
	if w.available("asr", filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing package available")
	}
}
