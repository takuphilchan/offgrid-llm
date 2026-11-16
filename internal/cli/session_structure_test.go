package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestSessionStructureCLIUsesPrivateRevisionAndAuthenticatedRead(t *testing.T) {
	id := "file-" + strings.Repeat("a", 32)
	invalid := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer isolated" || r.URL.Query().Get("revision") != "attachment-revision" || r.URL.Query().Get("offset") != "100" || r.URL.Query().Get("relationship_offset") != "2000" || r.URL.Query().Get("navigation_offset") != "100" || !strings.Contains(r.URL.Path, "/chat & policy/attachments/"+id+"/structure") {
			t.Error("lost private request identity")
		}
		_ = json.NewEncoder(w).Encode(document.StructurePage{DocumentID: id, Revision: "extraction-one", Offset: 100, RelationshipOffset: 2000, NavigationOffset: 100, Navigation: []document.NavigationEntry{}, CanCorrect: invalid, Blocks: []document.BlockView{}})
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "isolated", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"attachment-structure", "chat & policy", id, "attachment-revision", "100", "2000", "100"}
	var out bytes.Buffer
	if err = RunSessionService(context.Background(), client, args, &out, true, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"revision":"extraction-one"`) {
		t.Fatal(out.String())
	}
	invalid = true
	out.Reset()
	if err = RunSessionService(context.Background(), client, args, &out, true, nil); err == nil || out.Len() != 0 {
		t.Fatal("invalid private receipt displayed", err)
	}
}
