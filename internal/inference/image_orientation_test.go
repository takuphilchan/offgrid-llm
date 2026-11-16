//go:build !llama

package inference

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func inferenceImageFixture(t *testing.T, orientation uint16) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := 0; i < 6; i++ {
		img.SetNRGBA(i%3, i/3, color.NRGBA{R: uint8(i + 1), A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if orientation != 0 {
		exif := make([]byte, 26)
		copy(exif, "II")
		binary.LittleEndian.PutUint16(exif[2:], 42)
		binary.LittleEndian.PutUint32(exif[4:], 8)
		binary.LittleEndian.PutUint16(exif[8:], 1)
		binary.LittleEndian.PutUint16(exif[10:], 0x112)
		binary.LittleEndian.PutUint16(exif[12:], 3)
		binary.LittleEndian.PutUint32(exif[14:], 1)
		binary.LittleEndian.PutUint16(exif[18:], orientation)
		chunk := make([]byte, len(exif)+12)
		binary.BigEndian.PutUint32(chunk, uint32(len(exif)))
		copy(chunk[4:], "eXIf")
		copy(chunk[8:], exif)
		binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
		data = append(append(append([]byte{}, data[:33]...), chunk...), data[33:]...)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func TestImageOrientationIsSharedByCountCompleteAndStream(t *testing.T) {
	var received []string
	var mu sync.Mutex
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			Messages []struct {
				Content []api.ChatContentPart `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		url := request.Messages[0].Content[1].ImageURL.URL
		received = append(received, url)
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:image/png;base64,"))
		if err != nil {
			t.Error(err)
			return
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Error(err)
			return
		}
		if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 3 {
			t.Error("backend received unrotated pixels")
		}
		if strings.HasSuffix(r.URL.Path, "input_tokens") {
			_, _ = io.WriteString(w, `{"input_tokens":32}`)
			return
		}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"fixture\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"fixture"},"finish_reason":"stop"}],"usage":{"prompt_tokens":32,"completion_tokens":1,"total_tokens":33}}`)
	}))
	defer backend.Close()
	engine := NewLlamaHTTPEngine(backend.URL)
	request := &api.ChatCompletionRequest{Messages: []api.ChatMessage{{Role: "user", Content: api.SessionContent("Read this", []api.SessionImage{{Name: "scan.png", DataURL: inferenceImageFixture(t, 6)}})}}}
	before, _ := json.Marshal(request)
	if _, err := engine.CountPromptTokens(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ChatCompletion(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := engine.ChatCompletionStreamRaw(context.Background(), request, func(json.RawMessage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(request)
	if !bytes.Equal(before, after) {
		t.Fatal("runtime mutated source request")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 3 || received[0] != received[1] || received[1] != received[2] {
		t.Fatal("count and generation projections differ")
	}
}

func TestImageOrientationPreservesSingleStepSource(t *testing.T) {
	request := singleStepFixture()
	request.Messages = append(request.Messages, api.ChatMessage{Role: "user", Content: api.SessionContent("Read this", []api.SessionImage{{Name: "scan.png", DataURL: inferenceImageFixture(t, 6)}})})
	before, _ := json.Marshal(request)
	body, err := marshalLlamaRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(request)
	if !bytes.Equal(before, after) {
		t.Fatal("single-step source mutated")
	}
	if bytes.Contains(body, []byte(inferenceImageFixture(t, 6))) || !bytes.Contains(body, []byte("image_url")) {
		t.Fatal("single-step orientation not projected")
	}
}
