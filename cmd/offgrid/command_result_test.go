package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestCommandErrorContract(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{usage("Missing run ID"), 2, "invalid_usage"},
		{errors.New("failure"), 1, "operation_failed"},
		{context.Canceled, 130, "cancelled"},
		{&serviceclient.Error{Code: "forbidden", Message: "Forbidden"}, 1, "forbidden"},
	} {
		var stdout, stderr bytes.Buffer
		status := reportCommandError(test.err, true, &stdout, &stderr)
		var result struct {
			Error serviceclient.Error `json:"error"`
		}
		if status != test.status || json.Unmarshal(stdout.Bytes(), &result) != nil || result.Error.Code != test.code || stderr.Len() != 0 {
			t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
		}
	}
}

func TestCLIProcessHelper(t *testing.T) {
	if os.Getenv("OFFGRID_CLI_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"offgrid"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(99)
}

func TestCLIProcessExitAndJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testing-key" {
			http.Error(w, "Unauthorized", 401)
			return
		}
		if r.URL.Path == "/v1/rag/status" {
			_, _ = io.WriteString(w, `{"enabled":true}`)
			return
		}
		http.Error(w, "Forbidden secret", 403)
	}))
	defer server.Close()
	for _, test := range []struct {
		args     []string
		status   int
		contains string
	}{
		{[]string{"agent", "status", "--json"}, 2, `"invalid_usage"`},
		{[]string{"agent", "status", "run-1", "--json"}, 1, `"forbidden"`},
		{[]string{"kb", "enable", "--json"}, 2, `"invalid_usage"`},
		{[]string{"kb", "status", "--json"}, 0, `"enabled":true`},
		{[]string{"kb", "list", "--json"}, 1, `"forbidden"`},
		{[]string{"kb", "clear", "--json"}, 2, `"invalid_usage"`},
		{[]string{"workspace", "help", "--json"}, 0, `"help"`},
		{[]string{"session", "help", "--json"}, 0, `"help"`},
		{[]string{"session", "show", "--json"}, 2, `"invalid_usage"`},
		{[]string{"session", "list", "--json"}, 1, `"forbidden"`},
		{[]string{"export-session", "private", "--json"}, 1, `"forbidden"`},
		{[]string{"workspace", "backup", "--json"}, 2, `"invalid_usage"`},
		{[]string{"workspace", "restore", "missing.zip", "--json"}, 2, `"invalid_usage"`},
	} {
		command := exec.Command(os.Args[0], append([]string{"-test.run=^TestCLIProcessHelper$", "--"}, test.args...)...)
		root := t.TempDir()
		command.Env = append(os.Environ(), "OFFGRID_CLI_HELPER=1", "OFFGRID_SERVER_URL="+server.URL, "OFFGRID_API_KEY=testing-key", "OFFGRID_DATA_DIR="+filepath.Join(root, "data"), "OFFGRID_MODELS_DIR="+filepath.Join(root, "models"))
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		status := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			status = exit.ExitCode()
		}
		if status != test.status || !json.Valid(stdout.Bytes()) || !strings.Contains(stdout.String(), test.contains) || stderr.Len() != 0 {
			t.Fatalf("%v: status=%d stdout=%q stderr=%q", test.args, status, stdout.String(), stderr.String())
		}
	}
}

func TestKnowledgeUploadAndClearFailClosed(t *testing.T) {
	uploaded := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Error("missing key")
		}
		if r.URL.Path == "/v1/documents/ingest" {
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			defer r.MultipartForm.RemoveAll()
			data, _ := io.ReadAll(file)
			if string(data) != "hello" || header.Filename != "notes.md" {
				t.Error("bad upload")
			}
			uploaded = true
			_, _ = io.WriteString(w, `{"success":true}`)
			return
		}
		http.Error(w, "not authorised", 403)
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "key", nil)
	file := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledgeCommand(context.Background(), client, []string{"add", file}, strings.NewReader(""), io.Discard); err != nil || !uploaded {
		t.Fatalf("upload: %v", err)
	}
	if _, err := knowledgeCommand(context.Background(), client, []string{"clear", "--yes"}, strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("clear reported success despite rejected listing")
	}
}

func TestKnowledgeDoesNotAcceptEmptyOrFalseSuccess(t *testing.T) {
	for _, action := range []string{"status", "list", "search", "enable", "disable", "remove", "add"} {
		for _, body := range []string{`{}`, `{"success":false}`, `{"error":"private server detail"}`} {
			var response map[string]any
			if err := json.Unmarshal([]byte(body), &response); err != nil {
				t.Fatal(err)
			}
			if err := validateKnowledgeResponse(action, response); err == nil {
				t.Fatalf("%s accepted %s", action, body)
			}
		}
	}
}

func TestPartialMutationErrorKeepsCommittedCount(t *testing.T) {
	err := fmt.Errorf("Stopped after deleting 2 documents: %w", &serviceclient.Error{Code: "forbidden", Message: "Permission denied"})
	var stdout, stderr bytes.Buffer
	if code := reportCommandError(err, true, &stdout, &stderr); code != 1 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout.String(), "deleting 2 documents") || !strings.Contains(stdout.String(), `"code":"forbidden"`) {
		t.Fatalf("lost partial outcome: %s", stdout.String())
	}
}
