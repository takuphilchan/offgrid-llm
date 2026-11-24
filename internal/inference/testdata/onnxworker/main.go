// An isolated protocol/process fixture. This is not ONNX or model inference.
package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	state := os.Getenv("HOME")
	if len(os.Args) == 2 && os.Args[1] == "--descendant" {
		_ = os.WriteFile(filepath.Join(state, "descendant.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
		for {
			time.Sleep(time.Hour)
		}
	}
	args := map[string]string{}
	for i := 1; i+1 < len(os.Args); i++ {
		if len(os.Args[i]) > 2 && os.Args[i][:2] == "--" {
			args[os.Args[i]] = os.Args[i+1]
			i++
		}
	}
	identity, _ := base64.RawURLEncoding.DecodeString(args["--descriptor"])
	var descriptor map[string]any
	if json.Unmarshal(identity, &descriptor) != nil {
		os.Exit(2)
	}
	settingsRaw, err := os.ReadFile(filepath.Join(args["--model"], "embedding.json"))
	if err != nil {
		os.Exit(2)
	}
	var settings struct {
		Dimensions int `json:"dimensions"`
	}
	if json.Unmarshal(settingsRaw, &settings) != nil {
		os.Exit(2)
	}
	_ = os.WriteFile(filepath.Join(state, "parent.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
	child := exec.Command(os.Args[0], "--descendant")
	child.Stdout = io.Discard
	child.Stderr = io.Discard
	if child.Start() != nil {
		os.Exit(3)
	}
	encoder, decoder := json.NewEncoder(os.Stdout), json.NewDecoder(os.Stdin)
	_ = encoder.Encode(map[string]any{"protocol_revision": "provider-v1", "type": "ready", "request_id": "", "sequence": 0, "descriptor": descriptor, "resources": map[string]any{"backend": "cpu", "device": "isolated-fixture", "memory_measured": false}})
	pending := ""
	for {
		var input struct {
			Operation      string `json:"operation"`
			CancellationID string `json:"cancellation_id"`
			Request        struct {
				RequestID string `json:"request_id"`
				Prompt    struct {
					Input []string `json:"input"`
				} `json:"prompt"`
			} `json:"request"`
		}
		if decoder.Decode(&input) != nil {
			return
		}
		switch input.Operation {
		case "health":
			_ = encoder.Encode(map[string]any{"protocol_revision": "provider-v1", "type": "ready", "request_id": "", "sequence": 0, "descriptor": descriptor, "resources": map[string]any{"backend": "cpu", "device": "isolated-fixture", "memory_measured": false}})
		case "generate":
			if len(input.Request.Prompt.Input) > 0 && input.Request.Prompt.Input[0] == "cancel" {
				pending = input.Request.RequestID
				continue
			}
			vectors := make([][]float32, len(input.Request.Prompt.Input))
			for i := range vectors {
				vectors[i] = make([]float32, settings.Dimensions)
				vectors[i][0] = 1
			}
			_ = encoder.Encode(map[string]any{"protocol_revision": "provider-v1", "type": "embeddings", "request_id": input.Request.RequestID, "sequence": 1, "embeddings": vectors})
			_ = encoder.Encode(map[string]any{"protocol_revision": "provider-v1", "type": "done", "request_id": input.Request.RequestID, "sequence": 2, "finish_reason": "stop", "usage": map[string]int{"prompt_tokens": len(vectors) * 2, "completion_tokens": 0, "total_tokens": len(vectors) * 2}})
		case "cancel":
			if pending != "" && pending == input.CancellationID {
				_ = encoder.Encode(map[string]any{"protocol_revision": "provider-v1", "type": "done", "request_id": pending, "sequence": 1, "finish_reason": "cancelled", "usage": map[string]int{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}})
				pending = ""
			}
		}
	}
}
