package audio

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func fixturePackages(t *testing.T) (*models.PackageStore, string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	store := models.NewPackageStore(root)
	entries, err := models.CuratedPackageResolutions()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, entry := range entries {
		m := entry.Manifest
		for i := range m.Artifacts {
			data := []byte(m.Artifacts[i].Role)
			m.Artifacts[i].Size = int64(len(data))
			m.Artifacts[i].SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		_, err := store.Import(context.Background(), m, func(_ context.Context, a models.PackageArtifact) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(a.Role)), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		ids[m.Architecture] = m.ID
	}
	return store, root, ids
}

func TestSpeechProfilesRouteByArchitectureNotFileSuffix(t *testing.T) {
	store, root, ids := fixturePackages(t)
	legacy, err := NewEngine(Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	legacy.whisperPath = "fixture-whisper"
	legacy.piperPath = "fixture-piper"
	router := NewPackageRuntime(store, root, t.TempDir(), legacy)
	defer router.Close()
	profiles, err := router.Profiles(context.Background())
	if err != nil || len(profiles) != 4 {
		t.Fatalf("%+v %v", profiles, err)
	}
	for _, profile := range profiles {
		want := profile.Architecture == "whisper" || profile.Architecture == "piper"
		if profile.Available != want || profile.SmokeTested {
			t.Fatalf("false readiness: %+v", profile)
		}
	}
	for _, architecture := range []string{"kokoro", "zipformer-streaming"} {
		capability := models.CapabilitySynthesis
		if architecture == "zipformer-streaming" {
			capability = models.CapabilityTranscription
		}
		_, err := router.selectProfile(context.Background(), ids[architecture], capability)
		if err == nil || !strings.Contains(err.Error(), "sherpa-onnx") {
			t.Fatalf("unsupported package silently rerouted: %v", err)
		}
	}
	if _, err := router.selectProfile(context.Background(), ids["piper"], models.CapabilityTranscription); err == nil {
		t.Fatal("TTS accepted as ASR")
	}
	selected, err := router.selectProfile(context.Background(), "unknown-model", models.CapabilitySynthesis)
	if err != nil || selected != nil {
		t.Fatal("explicit unknown model fell back to another package")
	}
}

func TestWhisperPiperPackageExecutionAndLeases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture; run in Linux CI")
	}
	python := testPython(t)
	store, root, ids := fixturePackages(t)
	dir := t.TempDir()
	tool := filepath.Join(dir, "speech-fixture")
	body := "#!" + python + "\n" + `import sys,json,wave
args=sys.argv
if '-m' in args:
 assert open(args[args.index('-m')+1]).read()=='weights'
 with open(args[args.index('-of')+1]+'.json','w') as f:
  json.dump({'transcription':[{'text':'fixture transcript'}]},f)
 print('diagnostics must not become transcript')
else:
 assert open(args[args.index('--model')+1]).read()=='weights'
 assert open(args[args.index('--config')+1]).read()=='config'
 with wave.open(args[args.index('--output_file')+1],'wb') as f:
  f.setnchannels(1); f.setsampwidth(2); f.setframerate(22050); f.writeframes(b'\0'*100)
`
	if err := os.WriteFile(tool, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	legacy, err := NewEngine(Config{DataDir: dir, WhisperPath: tool, PiperPath: tool})
	if err != nil {
		t.Fatal(err)
	}
	router := NewPackageRuntime(store, root, dir, legacy)
	defer router.Close()
	result, err := router.Transcribe(context.Background(), TranscriptionRequest{Model: ids["whisper"], File: strings.NewReader("fixture"), Filename: "test.wav"})
	if err != nil || result.Text != "fixture transcript" {
		t.Fatalf("%+v %v", result, err)
	}
	profiles, _ := router.Profiles(context.Background())
	for _, profile := range profiles {
		if profile.ID == ids["whisper"] {
			if !profile.SmokeTested {
				t.Fatal("successful adapter invocation not recorded")
			}
			if err := store.Remove(profile.ID, profile.Revision); err != models.ErrPackageInUse {
				t.Fatalf("leased package removed: %v", err)
			}
		}
	}
	wav, err := router.Speak(context.Background(), SpeechRequest{Model: ids["piper"], Input: "test", ResponseFormat: "wav"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(wav)
	if len(b) <= 44 || string(b[:4]) != "RIFF" {
		t.Fatal("Piper did not return WAV")
	}
	for _, profile := range profiles {
		if profile.ID == ids["whisper"] {
			if err := store.Remove(profile.ID, profile.Revision); err != nil {
				t.Fatalf("old package lease not released: %v", err)
			}
		}
	}
}

func TestManagedPiperSpeakerSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture; run in Linux CI")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "speaker-fixture")
	body := "#!" + testPython(t) + "\n" + `import sys,wave
args=sys.argv
assert args[args.index('--speaker')+1]=='1'
with wave.open(args[args.index('--output_file')+1],'wb') as f:
 f.setnchannels(1); f.setsampwidth(2); f.setframerate(22050); f.writeframes(b'\0'*100)
`
	if err := os.WriteFile(tool, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "model.onnx", "fixture")
	writeFixture(t, dir, "model.json", `{"num_speakers":2}`)
	engine, err := NewEngine(Config{DataDir: dir, PiperPath: tool, Runtime: RuntimeConfig{PiperModelPath: filepath.Join(dir, "model.onnx"), PiperConfigPath: filepath.Join(dir, "model.json")}})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	for _, voice := range []string{"-1", "2", "1junk", "01", "../other"} {
		if _, err := engine.SpeakContext(context.Background(), SpeechRequest{Input: "fixture", Voice: voice}); err == nil || !strings.Contains(err.Error(), "speaker is unavailable") {
			t.Fatalf("accepted invalid speaker %q: %v", voice, err)
		}
	}
	wav, err := engine.SpeakContext(context.Background(), SpeechRequest{Input: "fixture", Voice: "1"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(wav)
	if err != nil || len(b) < 44 || string(b[:4]) != "RIFF" {
		t.Fatal("speaker fixture did not return WAV", err)
	}
}
