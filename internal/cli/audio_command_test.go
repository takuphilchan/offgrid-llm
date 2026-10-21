package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
	"github.com/takuphilchan/offgrid-llm/internal/config"
)

type fixtureAudioEngine struct{ whisper, piper int }

func (*fixtureAudioEngine) HasPiperBinary() bool { return true }
func (e *fixtureAudioEngine) DownloadWhisperModel(string, audio.DownloadProgressFunc) error {
	e.whisper++
	return nil
}
func (e *fixtureAudioEngine) DownloadPiperVoice(string, audio.DownloadProgressFunc) error {
	e.piper++
	return nil
}
func (*fixtureAudioEngine) DownloadPiperBinary(audio.DownloadProgressFunc) error { return nil }

func TestAudioUsesInjectedHTTPFilesAndSetupEngine(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "recording.wav")
	output := filepath.Join(root, "speech.wav")
	if err := os.WriteFile(input, []byte("fixture audio"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ServerPort: 11999} }
	engine := &fixtureAudioEngine{}
	transport := localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "localhost:11999" {
			t.Fatalf("wrong audio server %s", req.URL)
		}
		body := ""
		switch req.URL.Path {
		case "/v1/audio/transcriptions":
			body = `{"text":"recognized fixture"}`
		case "/v1/audio/speech":
			body = "WAVE"
		case "/v1/audio/status":
			body = `{"asr":{"available":true,"models":[]},"tts":{"available":true,"voices":1}}`
		case "/v1/audio/models":
			body = `{"whisper":{"installed":[],"available":[]},"piper":{"installed":[],"available":[]}}`
		default:
			t.Fatalf("unexpected audio endpoint %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	client := &http.Client{Transport: transport}
	env.Audio = AudioDependencies{
		Stat: os.Stat, ReadFile: os.ReadFile,
		Create: func(path string) (io.WriteCloser, error) {
			if path != output {
				t.Fatalf("wrong speech output %q", path)
			}
			return os.Create(path)
		},
		LongClient: client, ShortClient: client, GetClient: client,
		NewEngine: func(audio.Config) (AudioEngine, error) { return engine, nil },
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"audio", "transcribe", input}, "recognized fixture"},
		{[]string{"audio", "speak", "hello", "--output", output}, "Speech generated"},
		{[]string{"audio", "status"}, "Audio System Status"},
		{[]string{"audio", "models"}, "Audio Models"},
		{[]string{"audio", "setup", "whisper", "--model", "base.en"}, "Whisper model"},
		{[]string{"audio", "setup", "piper", "--voice", "en_US-amy-medium"}, "Piper voice"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v failed: code=%d out=%q err=%q", tc.args, code, out.String(), errOut.String())
		}
	}
	if data, err := os.ReadFile(output); err != nil || string(data) != "WAVE" {
		t.Fatalf("speech output mismatch: %v", err)
	}
	if engine.whisper != 1 || engine.piper != 1 {
		t.Fatalf("setup not routed to fake engine: %#v", engine)
	}
}
