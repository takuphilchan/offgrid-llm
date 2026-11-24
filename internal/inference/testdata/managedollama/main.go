// An isolated protocol/process fixture, never an inference implementation.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "hold" || os.Args[1] == "hold-leaf") {
		if runtime.GOOS == "linux" && os.Args[1] == "hold" {
			child := exec.Command(os.Args[0], "hold-leaf")
			child.Env = os.Environ()
			if child.Start() != nil {
				os.Exit(8)
			}
			reportIdentity("child")
		} else {
			reportIdentity("grandchild")
		}
		for {
			time.Sleep(time.Second)
		}
	}
	if len(os.Args) != 2 || os.Args[1] != "serve" {
		os.Exit(2)
	}
	state := os.Getenv("HOME")
	keys := []string{"USERPROFILE", "LOCALAPPDATA", "APPDATA", "TEMP", "TMP"}
	if runtime.GOOS == "linux" {
		keys = []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "TMPDIR", "TEMP", "TMP"}
	}
	for _, key := range keys {
		if os.Getenv(key) != state {
			os.Exit(3)
		}
	}
	if os.Getenv("OFFGRID_OLLAMA_SHOULD_NOT_INHERIT") != "" || os.Getenv("OLLAMA_NO_CLOUD") != "1" || os.Getenv("OLLAMA_NOPRUNE") != "1" || os.Getenv("OLLAMA_NUM_PARALLEL") != "1" || os.Getenv("OLLAMA_MAX_LOADED_MODELS") != "1" {
		os.Exit(4)
	}
	var model string
	var digest string
	_ = filepath.WalkDir(filepath.Join(os.Getenv("OLLAMA_MODELS"), "manifests"), func(path string, item os.DirEntry, err error) error {
		if err != nil || item.IsDir() {
			return err
		}
		model = filepath.Base(filepath.Dir(path)) + ":" + filepath.Base(path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		digest = hex.EncodeToString(hash[:])
		return nil
	})
	_ = os.WriteFile(filepath.Join(state, "root.pid"), []byte(fmt.Sprint(os.Getpid())), 0600)
	reportIdentity("parent")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			fmt.Fprint(w, `{"version":"fixture-1"}`)
		case "/api/tags":
			fmt.Fprintf(w, `{"models":[{"name":%q,"digest":%q}]}`, model, digest)
		case "/api/chat":
			var input struct {
				Model    string `json:"model"`
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.Model != model {
				http.Error(w, "invalid", 400)
				return
			}
			child := exec.Command(os.Args[0], "hold")
			child.Env = os.Environ()
			if child.Start() != nil {
				http.Error(w, "child failed", 500)
				return
			}
			_ = os.WriteFile(filepath.Join(state, "child.pid"), []byte(fmt.Sprint(child.Process.Pid)), 0600)
			for _, message := range input.Messages {
				if strings.Contains(message.Content, "block") {
					<-r.Context().Done()
					return
				}
			}
			fmt.Fprintf(w, "{\"model\":%q,\"message\":{\"role\":\"assistant\",\"content\":\"owned fixture\"},\"done\":true,\"done_reason\":\"stop\",\"prompt_eval_count\":3,\"eval_count\":2}\n", model)
		default:
			http.NotFound(w, r)
		}
	})
	server := &http.Server{Addr: os.Getenv("OLLAMA_HOST"), Handler: handler, ReadHeaderTimeout: time.Second}
	if server.ListenAndServe() != nil {
		os.Exit(5)
	}
}

func reportIdentity(role string) {
	if runtime.GOOS != "linux" {
		return
	}
	connection, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: filepath.Join(os.Getenv("HOME"), "observer.sock"), Net: "unixgram"})
	if err == nil {
		_, _ = connection.Write([]byte(role))
		connection.Close()
	}
}
