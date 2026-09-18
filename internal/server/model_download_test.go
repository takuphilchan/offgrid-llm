package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCancelModelDownloadRequiresExactName(t *testing.T) {
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	server := &Server{
		downloadCancelFuncs: map[string]context.CancelFunc{"tinyllama.gguf": cancelFirst, "tinyllama-large.gguf": cancelSecond},
		downloadProgress:    map[string]*DownloadProgress{"tinyllama.gguf": {Status: "finalizing"}, "tinyllama-large.gguf": {Status: "downloading"}},
	}
	server.handleCancelDownload(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(`{"file_name":"tinyllama"}`)))
	if first.Err() != nil || second.Err() != nil {
		t.Fatal("ambiguous substring cancelled a download")
	}
	server.handleCancelDownload(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(`{"file_name":"tinyllama.gguf"}`)))
	if first.Err() == nil || second.Err() != nil {
		t.Fatal("cancel did not isolate the exact download")
	}
}
