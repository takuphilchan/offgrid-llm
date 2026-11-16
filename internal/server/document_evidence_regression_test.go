package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func policyWithFootnote(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, content := range map[string]string{
		"word/document.xml":  `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Applications close on 30 June.</w:t><w:footnoteReference w:id="1"/></w:r></w:p></w:body></w:document>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:footnote w:id="1"><w:p><w:r><w:t>Rural applicants may apply until 31 July.</w:t></w:r></w:p></w:footnote></w:footnotes>`,
	} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// Exercises real parsing, HTTP upload/list/preview, saved references and the
// exact model input. The model response is deliberately a transport fixture;
// this test makes no claim about a real model's answer quality.
func TestDocumentEvidenceThroughConversationHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, question, text, want string
		docx                       bool
	}{
		{name: "policy.docx", question: "When can rural applicants apply?", want: "31 July", docx: true},
		{name: "records.txt", question: "青岚项目预算是多少？", text: strings.Repeat("普通会议记录。\n", 12000) + "青岚项目预算为七百万元。\n" + strings.Repeat("其他会议记录。\n", 12000), want: "七百万元"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newStreamingSessions(t)
			defer h.closeTurns()
			data := []byte(tc.text)
			if tc.docx {
				data = policyWithFootnote(t)
			}
			w := uploadChatFile(t, h, tc.name, string(data))
			var result struct {
				Attachment sessions.Attachment `json:"attachment"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
				t.Fatal(w.Code, w.Body.String())
			}
			a := result.Attachment
			if a.Evidence == nil || a.Evidence.TotalUnits != 1 || (tc.docx && (a.Evidence.Status != "text_only" || len(a.Evidence.Warnings) == 0)) {
				t.Fatalf("lost extraction receipt: %+v", a)
			}
			for _, path := range []string{"/v1/sessions/chat/attachments", "/v1/sessions/chat/attachments/" + a.ID + "/preview"} {
				w = httptest.NewRecorder()
				h.HandleSessions(w, httptest.NewRequest("GET", path, nil))
				if w.Code != 200 || !strings.Contains(w.Body.String(), `"evidence"`) {
					t.Fatal("read receipt", w.Code, w.Body.String())
				}
			}
			calls := 0
			h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
				calls++
				input := req.Messages[len(req.Messages)-1].StringContent()
				if !strings.Contains(input, tc.want) || !strings.Contains(input, `"evidence"`) {
					t.Errorf("missing evidence in model input: %s", input)
				}
				if tc.docx && !strings.Contains(input, "document_text_only") {
					t.Error("missing warning in context")
				}
				return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
			}
			for i := 0; i < 2; i++ {
				request := sessionGenerateRequest{Content: tc.question, Stream: true, Durable: true, RequestID: []string{"evidence-first", "evidence-followup"}[i]}
				if i == 0 {
					request.Attachments = []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}
				}
				body, _ := json.Marshal(request)
				w = httptest.NewRecorder()
				h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)))
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
			}
			if calls != 2 {
				t.Fatal("generation was not exercised", calls)
			}
		})
	}
}
