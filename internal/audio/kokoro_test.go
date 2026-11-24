package audio

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func fixtureKokoro(t *testing.T, modern bool) (*managedWorker, string) {
	t.Helper()
	w, dir := fixtureWorker(t)
	modules := os.Getenv("PYTHONPATH")
	script := `import os,time
print("sherpa diagnostic",flush=True)
class OfflineTtsKokoroModelConfig:
 def __init__(self,**kw):
  assert kw['model'] and kw['voices'] and kw['tokens'] and kw['data_dir']
class OfflineTtsModelConfig:
 def __init__(self,**kw):
  assert kw['provider']=='cpu' and kw['num_threads']==4 and kw['debug'] is False
class OfflineTtsConfig:
 def __init__(self,**kw): assert kw['max_num_sentences']==1
 def validate(self): return True
class Audio:
 samples=[0,0,0,0]
 sample_rate=24000
class OfflineTts:
 num_speakers=2
 def __init__(self,config): pass
 def generate(self,text,*args,**kw):
  sid=args[0].sid if args else kw['sid']
  assert sid in (0,1)
  if text=='wait': time.sleep(10)
  if text=='fail': raise ValueError('PRIVATE TEXT MUST NOT LEAK')
  return Audio()
`
	if modern {
		script += "__version__='2.0.0'\nclass GenerationConfig: pass\n"
	} else {
		script += "__version__='1.12.20'\n"
	}
	version := "1.12.20"
	if modern {
		version = "2.0.0"
	}
	metadata := filepath.Join(modules, "sherpa_onnx-"+version+".dist-info")
	if err := os.MkdirAll(metadata, 0700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, metadata, "METADATA", "Name: sherpa-onnx\nVersion: "+version+"\n")
	writeFixture(t, modules, "sherpa_onnx.py", script)
	w.cfg.Kokoro = &KokoroConfig{Model: "model.onnx", Voices: "voices.bin", Tokens: "tokens.txt", DataDir: "espeak-ng-data", Lexicon: "lexicon-en.txt", VoiceIDs: []string{"0", "1", "99"}}
	return w, dir
}

func TestKokoroModernWorkerRejectsEspeakOnlyPackage(t *testing.T) {
	w, dir := fixtureKokoro(t, true)
	w.cfg.Kokoro.Lexicon = ""
	if _, err := w.speak(context.Background(), SpeechRequest{Input: "hello"}, dir); err == nil || !strings.Contains(err.Error(), "requires Kokoro package lexicons") {
		t.Fatalf("espeak-only package was not rejected: %v", err)
	}
}

func TestKokoroLegacyWorkerAcceptsEspeakOnlyPackage(t *testing.T) {
	w, dir := fixtureKokoro(t, false)
	w.cfg.Kokoro.Lexicon = ""
	if _, err := w.speak(context.Background(), SpeechRequest{Input: "hello"}, dir); err != nil {
		t.Fatal(err)
	}
}

func TestKokoroWorkerRejectsUnreviewedVersion(t *testing.T) {
	w, dir := fixtureKokoro(t, false)
	file := filepath.Join(os.Getenv("PYTHONPATH"), "sherpa_onnx.py")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(string(b), "1.12.20", "9.0.0")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.speak(context.Background(), SpeechRequest{Input: "hello"}, dir); err == nil || !strings.Contains(err.Error(), "reviewed sherpa-onnx") {
		t.Fatalf("unreviewed runtime accepted: %v", err)
	}
}

func TestKokoroReadinessRequiresReviewedRuntimeAndFrontend(t *testing.T) {
	m := models.ModelPackageManifest{}
	modules := map[string]bool{"sherpa_onnx": true, "soundfile": true}
	if kokoroRuntimeAvailable(modules, m) {
		t.Fatal("unreviewed version ready")
	}
	modules["kokoro_lexicon"] = true
	if kokoroRuntimeAvailable(modules, m) {
		t.Fatal("modern runtime ready without lexicon")
	}
	modules["kokoro_legacy"] = true
	if !kokoroRuntimeAvailable(modules, m) {
		t.Fatal("legacy runtime rejected")
	}
	modules["kokoro_legacy"] = false
	m.Artifacts = []models.PackageArtifact{{Role: "phonemizer_data", Path: "lexicon-en.txt"}}
	if !kokoroRuntimeAvailable(modules, m) {
		t.Fatal("modern lexicon package rejected")
	}
}

func TestKokoroWorkerSpeechSelectionAndCancellation(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "generation-config"}[modern], func(t *testing.T) {
			w, dir := fixtureKokoro(t, modern)
			for _, voice := range []string{"", "1"} {
				out, err := w.speak(context.Background(), SpeechRequest{Input: "hello", Voice: voice}, dir)
				if err != nil {
					t.Fatal(err)
				}
				data, _ := io.ReadAll(out)
				if len(data) < 44 || string(data[:4]) != "RIFF" {
					t.Fatal("no WAV")
				}
			}
			for _, voice := range []string{"../outside", "99"} {
				if _, err := w.speak(context.Background(), SpeechRequest{Input: "hello", Voice: voice}, dir); err == nil {
					t.Fatal("invalid voice accepted")
				}
			}
			_, err := w.speak(context.Background(), SpeechRequest{Input: "fail"}, dir)
			if err == nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("unsafe error: %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			_, err = w.speak(ctx, SpeechRequest{Input: "wait"}, dir)
			if !errors.Is(err, context.DeadlineExceeded) || w.proc != nil {
				t.Fatalf("worker not cancelled: %v", err)
			}
		})
	}
}

func TestKokoroPackageRoutingAndLease(t *testing.T) {
	w, _ := fixtureKokoro(t, false)
	t.Setenv("OFFGRID_SPEECH_PYTHON", w.cfg.PythonPath)
	store, root, ids := fixturePackages(t)
	legacy, err := NewEngine(Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	router := NewPackageRuntime(store, root, t.TempDir(), legacy)
	defer router.Close()
	profile, err := router.selectProfile(context.Background(), ids["kokoro"], models.CapabilitySynthesis)
	if err != nil || profile == nil {
		t.Fatalf("Kokoro unavailable: %v", err)
	}
	if !router.RuntimeAvailable(context.Background(), *profile.manifest) {
		t.Fatal("Kokoro preflight unavailable")
	}
	other := *profile.manifest
	other.Architecture = "zipformer-streaming"
	if router.RuntimeAvailable(context.Background(), other) {
		t.Fatal("Kokoro runtime enabled Zipformer")
	}
	if _, err = router.Speak(context.Background(), SpeechRequest{Input: "hello", Model: profile.ID}); err != nil {
		t.Fatal(err)
	}
	if err = store.Remove(profile.ID, profile.Revision); err == nil {
		t.Fatal("loaded package removed")
	}
	router.Close()
	if err = store.Remove(profile.ID, profile.Revision); err != nil {
		t.Fatal(err)
	}
}
