package cli

import (
	"io"
	"net/http"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
)

type AudioEngine interface {
	HasPiperBinary() bool
	DownloadWhisperModel(string, audio.DownloadProgressFunc) error
	DownloadPiperVoice(string, audio.DownloadProgressFunc) error
	DownloadPiperBinary(audio.DownloadProgressFunc) error
}

type AudioDependencies struct {
	Stat        func(string) (os.FileInfo, error)
	ReadFile    func(string) ([]byte, error)
	Create      func(string) (io.WriteCloser, error)
	LongClient  *http.Client
	ShortClient *http.Client
	GetClient   *http.Client
	NewEngine   func(audio.Config) (AudioEngine, error)
}
