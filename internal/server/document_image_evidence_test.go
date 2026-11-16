package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestImageDocumentHTTPMissingPackRetainsInspectableOriginal(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewGray(image.Rect(0, 0, 100, 80))); err != nil {
		t.Fatal(err)
	}
	w := uploadChatFile(t, h, "record.png", data.String())
	var upload struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 422 || json.Unmarshal(w.Body.Bytes(), &upload) != nil || upload.Attachment.ID == "" || upload.Attachment.Evidence == nil || upload.Attachment.Evidence.Status != "partial" {
		t.Fatal(w.Code, w.Body.String())
	}
	path := "/v1/sessions/chat/attachments/" + upload.Attachment.ID
	for _, tc := range []struct {
		suffix string
		code   int
	}{{"/preview", 200}, {"/preview?page=1", 200}, {"/preview?page=2", 400}, {"/original", 200}, {"/preview?sha256=" + strings.Repeat("0", 64), 409}} {
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", path+tc.suffix, nil))
		if w.Code != tc.code {
			t.Fatal(tc.suffix, w.Code, w.Body.String())
		}
		if tc.suffix == "/original" && !bytes.Equal(w.Body.Bytes(), data.Bytes()) {
			t.Fatal("original modified")
		}
		if tc.suffix == "/preview?page=1" {
			if _, err := png.Decode(bytes.NewReader(w.Body.Bytes())); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A partial OCR result remains inspectable without granting generation.
	scope := h.scoped(httptest.NewRequest("GET", "/", nil))
	evidence := &document.Coverage{Format: "image_document", Status: "partial", TotalUnits: 1, ReviewUnits: []int{1}}
	if _, err := scope.PrepareAttachmentEvidence("chat", upload.Attachment.ID, "uncertain amount 42", "image-test", "document_ocr_required", "", evidence); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", path+"/preview", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "uncertain amount 42") || !strings.Contains(w.Body.String(), `"state":"partial"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, _, err := scope.ReadAttachment("chat", upload.Attachment.ID); err == nil {
		t.Fatal("review-only source admitted")
	}
	// Explicit acknowledgement reuses uncertain OCR; no configured worker or
	// language pack is needed, and the extraction coverage stays partial.
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", path+"/prepare?text_only=true", nil))
	var accepted struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &accepted) != nil || accepted.Attachment.Coverage != "partial_text_only" || accepted.Attachment.Evidence.Status != "partial" {
		t.Fatal(w.Code, w.Body.String())
	}
	_, text, err := scope.ReadAttachment("chat", upload.Attachment.ID)
	if err != nil || text != "uncertain amount 42" {
		t.Fatal("acknowledgement erased OCR", text, err)
	}
	sources, err := scope.AttachmentSources("chat", []sessions.AttachmentReference{{ID: accepted.Attachment.ID, Revision: accepted.Attachment.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	prompt := sessions.AttachmentPrompt("What is the amount?", sources)
	if !strings.Contains(prompt, "uncertain OCR") || !strings.Contains(prompt, `"status":"partial"`) || !strings.Contains(prompt, "uncertain amount 42") {
		t.Fatal("model uncertainty lost", prompt)
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", path+"/preview", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"partial"`) || !strings.Contains(w.Body.String(), "uncertain amount 42") {
		t.Fatal(w.Code, w.Body.String())
	}
}

// Compiled next to the packaged worker, this exercises actual OCR through HTTP,
// saved private structure, generation context and source reopening. No vision or
// embedding model is required. The model answer itself is a transport fixture.
func TestRealImageDocumentPrivateHTTP(t *testing.T) {
	fixture, language := os.Getenv("OFFGRID_TEST_IMAGE_DOCUMENT"), os.Getenv("OFFGRID_TEST_OCR_LANGUAGE")
	if fixture == "" || language == "" {
		t.Skip("requires explicit local image and language fixtures plus app-local OCR worker")
	}
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	filename := "receipt.png"
	if os.Getenv("OFFGRID_TEST_TIFF") == "1" {
		source = runtimeTIFF(t, source)
		filename = "records.tiff"
	}
	pack, err := os.ReadFile(language)
	if err != nil {
		t.Fatal(err)
	}
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	if err = e.ImportOCRLanguage(context.Background(), "eng", bytes.NewReader(pack)); err != nil {
		t.Fatal(err)
	}
	h := newStreamingSessions(t)
	defer h.closeTurns()
	h.agentServer = &Server{ragEngine: e}
	w := uploadChatFile(t, h, filename, string(source))
	var upload struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &upload) != nil || upload.Attachment.ExtractionRevision == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	a := upload.Attachment
	path := "/v1/sessions/chat/attachments/" + a.ID
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", path+"/structure?revision="+a.Revision, nil))
	var graph document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &graph) != nil || graph.Method != "ocr" || len(graph.Blocks) == 0 || graph.Blocks[0].Bounds == nil {
		t.Fatal(w.Code, w.Body.String())
	}
	// Preview must reopen retained text even with no OCR-configured engine.
	h.agentServer = nil
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", path+"/preview", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Scanned receipt total 42") || !strings.Contains(w.Body.String(), `"status":"text_only"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	calls := 0
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		input := req.Messages[len(req.Messages)-1].StringContent()
		if !strings.Contains(input, "Scanned receipt total 42") || !strings.Contains(input, "text_only") {
			t.Fatal("missing OCR evidence or limits", input)
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	body, _ := json.Marshal(sessionGenerateRequest{Content: "What is the receipt total?", Stream: true, Durable: true, RequestID: "image-evidence", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}})
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)))
	if w.Code != 200 || calls != 1 {
		t.Fatal(w.Code, w.Body.String(), calls)
	}
	saved, err := h.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Messages[len(saved.Messages)-1].AttachmentSelections) != 1 {
		t.Fatal("source receipt not saved")
	}
}
