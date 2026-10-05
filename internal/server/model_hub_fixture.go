//go:build modeltestfixtures

package server

// This transport is deliberately absent from production builds. It exercises
// the real resolver, transfer, storage, HTTP service and renderer with synthetic
// data. It cannot access an external network or enable inference.
import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

const integrationModelRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type modelFixtureTransport struct{}
type fixtureModelFile struct {
	bytes  []byte
	weight bool
}

func fixtureModelFiles(arch string) map[string]fixtureModelFile {
	files := map[string]fixtureModelFile{}
	add := func(path, body string, weight bool) {
		if weight {
			body = strings.Repeat("SYNTHETIC NOT AN INFERENCE MODEL ", 8192)
		}
		files[path] = fixtureModelFile{[]byte(body), weight}
	}
	switch arch {
	case "whisper":
		add("ggml-tiny.en.bin", "", true)
	case "piper":
		add("en/voice.onnx", "", true)
		add("en/voice.onnx.json", `{"audio":{"sample_rate":22050},"language":{"code":"en_US"},"num_speakers":1,"phoneme_type":"espeak"}`, false)
		add("en/MODEL_CARD", "CC0-1.0 synthetic fixture; not a usable voice", false)
	case "zipformer-streaming":
		for _, role := range []string{"encoder", "decoder", "joiner"} {
			add(role+"-epoch-99-avg-1-chunk-16-left-128.int8.onnx", "", true)
		}
		add("tokens.txt", "token 0", false)
	case "kokoro":
		add("model.onnx", "", true)
		add("voices.bin", "", true)
		add("tokens.txt", "token 0", false)
		add("LICENSE", "Apache License\nVersion 2.0", false)
		for _, path := range []string{"phondata", "phonindex", "phontab", "en_dict", "lang/gmw/en"} {
			add("espeak-ng-data/"+path, "synthetic "+path, false)
		}
	}
	return files
}

type fixtureSlowBody struct {
	ctx    context.Context
	reader *bytes.Reader
}

func (b *fixtureSlowBody) Close() error { return nil }
func (b *fixtureSlowBody) Read(p []byte) (int, error) {
	timer := time.NewTimer(60 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	case <-timer.C:
	}
	if len(p) > 8192 {
		p = p[:8192]
	}
	return b.reader.Read(p)
}
func (modelFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "huggingface.co" || r.Header.Get("Authorization") != "" {
		return nil, fmt.Errorf("fixture forbids external network or credentials")
	}
	respond := func(status int, data []byte) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data)), Request: r}
	}
	if r.URL.Path == "/api/models" {
		entries := []map[string]any{}
		for _, arch := range []string{"whisper", "piper", "zipformer-streaming", "kokoro"} {
			entries = append(entries, map[string]any{"id": "fixture/" + arch, "modelId": "fixture/" + arch, "downloads": 1, "likes": 0, "sha": integrationModelRevision, "tags": []string{"test-fixture"}, "siblings": []any{}})
		}
		b, _ := json.Marshal(entries)
		return respond(200, b), nil
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/models/")
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] != "fixture" {
		return respond(404, []byte(`{}`)), nil
	}
	repo, arch := parts[0]+"/"+parts[1], parts[1]
	files := fixtureModelFiles(arch)
	if len(files) == 0 {
		return respond(404, []byte(`{}`)), nil
	}
	if strings.Contains(r.URL.Path, "/tree/") {
		entries := []map[string]any{}
		for path, f := range files {
			git := sha1.New()
			fmt.Fprintf(git, "blob %d\x00", len(f.bytes))
			git.Write(f.bytes)
			e := map[string]any{"type": "file", "path": path, "size": len(f.bytes), "oid": hex.EncodeToString(git.Sum(nil))}
			if f.weight {
				digest := sha256.Sum256(f.bytes)
				e["lfs"] = map[string]any{"oid": hex.EncodeToString(digest[:]), "size": len(f.bytes)}
			}
			entries = append(entries, e)
		}
		b, _ := json.Marshal(entries)
		return respond(200, b), nil
	}
	if strings.HasPrefix(r.URL.Path, "/api/models/") {
		b, _ := json.Marshal(map[string]any{"id": repo, "sha": integrationModelRevision, "private": false, "gated": false, "cardData": map[string]any{"license": "CC0-1.0", "language": []string{"en"}}})
		return respond(200, b), nil
	}
	prefix := "/" + repo + "/resolve/" + integrationModelRevision + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return respond(404, []byte(`{}`)), nil
	}
	file, ok := files[strings.TrimPrefix(r.URL.Path, prefix)]
	if !ok {
		return respond(404, []byte(`{}`)), nil
	}
	offset := 0
	status := 200
	if value := r.Header.Get("Range"); value != "" {
		if _, err := fmt.Sscanf(value, "bytes=%d-", &offset); err != nil || offset < 0 || offset >= len(file.bytes) {
			response := respond(416, nil)
			response.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", len(file.bytes)))
			return response, nil
		}
		status = 206
	}
	response := respond(status, file.bytes[offset:])
	if status == 206 {
		response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(file.bytes)-1, len(file.bytes)))
	}
	if file.weight {
		response.Body = &fixtureSlowBody{r.Context(), bytes.NewReader(file.bytes[offset:])}
	}
	return response, nil
}
func newModelHub() *models.HuggingFaceClient {
	return models.NewHuggingFaceClient().WithHTTPClient(&http.Client{Transport: modelFixtureTransport{}})
}
