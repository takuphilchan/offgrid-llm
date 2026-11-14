package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationChangesReadOnlyPagingAndIntegrity(t *testing.T) {
	data := []byte("Full proposed contents with final citations")
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	mode := ""
	requests := 0
	change := api.WorkChange{ID: "change", WorkID: "work", State: "partial"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 120 {
			t.Error("unbounded paging")
			w.WriteHeader(500)
			return
		}
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("inspection changed authority or dispatched", r.Method)
		}
		switch r.URL.Path {
		case "/v1/sessions/Saved":
			id := "conversation"
			if mode == "legacy" {
				id = ""
			}
			json.NewEncoder(w).Encode(sessions.Session{Name: "Saved", CanonicalID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		case "/api/v2/conversations/conversation":
			json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", FocusedWorkID: "work"})
		case "/api/v2/work-items/work/changes":
			cursor := r.URL.Query().Get("after")
			item := change
			item.ID = "change-a"
			if cursor != "" {
				item.ID = "change-b"
			}
			page := api.WorkChangeList{Items: []api.WorkChange{item}, NextCursor: item.ID, HasMore: cursor == ""}
			if mode == "loop" {
				page.NextCursor = cursor
				page.HasMore = true
			}
			if mode == "foreign" {
				page.Items[0].WorkID = "foreign"
			}
			json.NewEncoder(w).Encode(page)
		case "/api/v2/work-items/work/changes/change":
			var offset int64
			fmt.Sscan(r.URL.Query().Get("offset"), &offset)
			view := r.URL.Query().Get("view")
			page := api.WorkChangeDetail{Change: change, View: view}
			if mode == "foreign" {
				page.Change.WorkID = "foreign"
			}
			if view == "content" || view == "preserved" {
				if view == "preserved" && r.URL.Query().Get("artifact") != "1:displaced" {
					t.Error("lost recovery identity")
				}
				if r.URL.Query().Get("hash") != hash || r.URL.Query().Get("limit") != "65536" {
					t.Error("lost content query")
				}
				end := min(offset+8, int64(len(data)))
				chunk := append([]byte(nil), data[offset:end]...)
				if mode == "corrupt" {
					chunk[0] ^= 1
				}
				page.Content = &api.WorkChangeContent{SHA256: hash, Bytes: int64(len(data)), Offset: offset, Data: chunk}
				if end < int64(len(data)) {
					page.NextOffset = &end
				}
			} else {
				page.Files = []api.WorkFileChange{{Path: fmt.Sprintf("file-%d", offset), Kind: "update"}}
				if offset == 0 {
					next := int64(1)
					page.NextOffset = &next
				}
			}
			if mode == "loop" {
				page.NextOffset = &offset
			}
			json.NewEncoder(w).Encode(page)
		default:
			t.Error("unexpected read", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "fixture", nil)
	var out bytes.Buffer
	ctx := context.Background()
	if err := runSessionChanges(ctx, client, []string{"changes", "Saved"}, &out, true); err != nil {
		t.Fatal(err)
	}
	var list []api.WorkChange
	if err := json.Unmarshal(out.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	if err := inspectConversationChanges(ctx, client, "conversation", []string{"change", "content", hash}, &out, true); err != nil {
		t.Fatal(err)
	}
	var got []byte
	decoder := json.NewDecoder(&out)
	for {
		var page api.WorkChangeDetail
		err := decoder.Decode(&page)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, page.Content.Data...)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("content lost", string(got))
	}
	out.Reset()
	if err := runSessionChanges(ctx, client, []string{"changes", "Saved", "change", "preserved", hash, "1:displaced"}, &out, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"view":"preserved"`) {
		t.Fatal("missing retained content", out.String())
	}
	for _, args := range [][]string{{"change", "preserved", hash}, {"change", "content", hash, "1:displaced"}} {
		if err := inspectConversationChanges(ctx, client, "conversation", args, &out, true); err == nil {
			t.Fatal("invalid recovery request", args)
		}
	}
	out.Reset()
	if err := inspectConversationChanges(ctx, client, "conversation", []string{"change"}, &out, true); err != nil || !strings.Contains(out.String(), "file-1") {
		t.Fatal(out.String(), err)
	}
	for _, bad := range []string{"loop", "foreign", "corrupt"} {
		mode = bad
		out.Reset()
		if err := inspectConversationChanges(ctx, client, "conversation", []string{"change", "content", hash}, &out, true); err == nil {
			t.Fatal("accepted invalid content", bad)
		}
		if bad != "corrupt" {
			if err := inspectConversationChanges(ctx, client, "conversation", nil, &out, true); err == nil {
				t.Fatal("accepted invalid list", bad)
			}
		}
	}
	mode = "legacy"
	before := requests
	if err := runSessionChanges(ctx, client, []string{"changes", "Saved"}, &out, true); err == nil || requests != before+1 {
		t.Fatal("legacy inspection migrated or dispatched", err)
	}
}
