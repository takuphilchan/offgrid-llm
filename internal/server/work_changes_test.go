package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workChangeHTTP(s *Server, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.handleControlWork(w, httptest.NewRequest("GET", path, nil))
	return w
}

func testWorkChangeInspection(t *testing.T, s *Server, root *agents.Task, source string) {
	t.Helper()
	base := "/api/v2/work-items/" + root.WorkID + "/changes"
	seen := map[string]bool{}
	var candidate api.WorkChange
	for cursor := ""; ; {
		w := workChangeHTTP(s, base+"?limit=1&after="+cursor)
		var page api.WorkChangeList
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) != 1 {
			t.Fatal(w.Code, w.Body.String())
		}
		item := page.Items[0]
		if seen[item.ID] || !item.Available || item.WorkID != root.WorkID {
			t.Fatal("invalid change cursor or owner", item)
		}
		seen[item.ID] = true
		record, err := s.agentManager.ChangeSetRecord(agents.ControlScope{Workspace: s.workspaceID, Actor: root.Actor}, item.ID)
		if err != nil || record.WorkID != root.WorkID || record.State != item.State || record.Revision < 1 || record.OperationsDigest == "" {
			t.Fatal("change identity was not committed with the run", record, err)
		}
		encoded, _ := json.Marshal(record)
		if strings.Contains(string(encoded), "fixture-local-grant") || record.Source.Grant != "" {
			t.Fatal("control record retained live host authority")
		}
		if item.Kind == "candidate" {
			candidate = item
		}
		if !page.HasMore {
			break
		}
		if page.NextCursor <= cursor {
			t.Fatal("nonadvancing cursor")
		}
		cursor = page.NextCursor
	}
	if len(seen) != 3 || candidate.State != "integrated" {
		t.Fatal("missing retained changes", seen, candidate)
	}
	var hash string
	for _, view := range []string{"files", "validation", "integration"} {
		w := workChangeHTTP(s, base+"/"+candidate.ID+"?view="+view+"&limit=1")
		var detail api.WorkChangeDetail
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil {
			t.Fatal(view, w.Code, w.Body.String())
		}
		if detail.Change.State != candidate.State {
			t.Fatal("inspection regressed integration state", view, detail.Change.State)
		}
		for _, forbidden := range []string{"fixture-local-grant", `"source":`, `"copy_id":`, `"command":`, `"displaced":`} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatal("private execution material exposed", forbidden)
			}
		}
		switch view {
		case "files":
			if len(detail.Files) != 1 || detail.Files[0].Path != "input.txt" {
				t.Fatal(detail)
			}
			hash = detail.Files[0].Result.SHA256
		case "validation":
			if len(detail.Checks) != 1 || detail.Checks[0].ExitCode == nil || *detail.Checks[0].ExitCode != 0 {
				t.Fatal(detail)
			}
		case "integration":
			if len(detail.Receipts) != 1 || !detail.Receipts[0].OriginalRetained || detail.Receipts[0].State != "integrated" {
				t.Fatal(detail)
			}
		}
	}
	var content []byte
	for offset := int64(0); ; {
		w := workChangeHTTP(s, base+"/"+candidate.ID+"?view=content&hash="+hash+"&limit=4&offset="+strconv.FormatInt(offset, 10))
		var detail api.WorkChangeDetail
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || detail.Content == nil || detail.Content.Offset != offset {
			t.Fatal(w.Code, w.Body.String())
		}
		content = append(content, detail.Content.Data...)
		if detail.NextOffset == nil {
			break
		}
		if *detail.NextOffset <= offset {
			t.Fatal("nonadvancing content cursor")
		}
		offset = *detail.NextOffset
	}
	if string(content) != "sealed editor result" {
		t.Fatal("content truncated", string(content))
	}
	w := workChangeHTTP(s, base+"/"+candidate.ID+"?view=recovery&limit=1")
	var recovery api.WorkChangeDetail
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &recovery) != nil || len(recovery.Recovery) != 1 || recovery.Recovery[0].Role != "displaced" {
		t.Fatal("missing recovery evidence", w.Code, w.Body.String())
	}
	file := recovery.Recovery[0]
	content = nil
	for offset := int64(0); ; {
		w = workChangeHTTP(s, base+"/"+candidate.ID+"?view=preserved&hash="+file.SHA256+"&artifact="+file.ID+"&limit=4&offset="+strconv.FormatInt(offset, 10))
		var detail api.WorkChangeDetail
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || detail.Content == nil || detail.Content.Offset != offset || detail.Content.SHA256 != file.SHA256 {
			t.Fatal("missing retained bytes", w.Code, w.Body.String())
		}
		content = append(content, detail.Content.Data...)
		if detail.NextOffset == nil {
			break
		}
		if *detail.NextOffset <= offset {
			t.Fatal("nonadvancing recovery cursor")
		}
		offset = *detail.NextOffset
	}
	if string(content) != "user's dirty input" {
		t.Fatal("lost retained user input", string(content))
	}
	for _, query := range []string{"?limit=0", "?limit=65", "?offset=-1", "?view=write"} {
		if w := workChangeHTTP(s, base+"/"+candidate.ID+query); w.Code != 400 {
			t.Fatal("invalid page accepted", query, w.Code)
		}
	}
	if w := workChangeHTTP(s, base+"/"+strings.Repeat("f", 64)); w.Code != 404 {
		t.Fatal("unknown change leaked", w.Code)
	}
	foreign := *s
	cfg := *s.config
	cfg.RequireAuth = true
	foreign.config = &cfg
	if w := workChangeHTTP(&foreign, base+"/"+candidate.ID); w.Code != 404 {
		t.Fatal("foreign actor read change", w.Code)
	}
	current, _ := s.agentManager.GetTask(root.ID)
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "sealed editor result" || len(current.Steps) != len(root.Steps) || current.Status != root.Status {
		t.Fatal("inspection changed work or source", err)
	}
}

func testRevokedWorkChangeInspection(t *testing.T, s *Server, root *agents.Task, id string) {
	t.Helper()
	base := "/api/v2/work-items/" + root.WorkID + "/changes"
	if w := workChangeHTTP(s, base+"/"+id); w.Code != 403 {
		t.Fatal("revoked source was readable", w.Code, w.Body.String())
	}
	for _, view := range []string{"recovery", "preserved"} {
		if w := workChangeHTTP(s, base+"/"+id+"?view="+view); w.Code != 403 {
			t.Fatal("revoked recovery readable", view, w.Code)
		}
	}
	w := workChangeHTTP(s, base)
	var page api.WorkChangeList
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) == 0 {
		t.Fatal("revocation hid saved status", w.Code, w.Body.String())
	}
	for _, item := range page.Items {
		if item.Available || item.Limitation == "" {
			t.Fatal("revoked access restored", item)
		}
	}
}
