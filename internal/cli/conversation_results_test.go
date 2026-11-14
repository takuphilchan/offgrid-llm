package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSavedResultCLIReadsAllPagesAndRejectsBrokenCursors(t *testing.T) {
	reference, content := "result:run:abc", strings.Repeat("Evidence. ", 2000)+"Final citation [42]."
	broken := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v2/results/"+reference {
			t.Error("unexpected dispatch", r.Method, r.URL.Path)
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := min(offset+10000, len(content))
		page := api.ResultPage{ID: reference, Offset: offset, NextOffset: end, TotalBytes: len(content), Content: content[offset:end], Complete: end == len(content)}
		if broken {
			page.NextOffset = offset
		}
		json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = inspectSavedResult(context.Background(), client, reference, &output, false); err != nil || output.String() != content+"\n" {
		t.Fatal("lost result", err)
	}
	broken = true
	output.Reset()
	if err = inspectSavedResult(context.Background(), client, reference, &output, true); err == nil || output.Len() != 0 {
		t.Fatal("accepted broken cursor", err)
	}
}
