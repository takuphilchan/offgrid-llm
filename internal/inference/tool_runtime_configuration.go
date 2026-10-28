package inference

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Hash the cache-owned child's actual launch configuration, not current mutable
// service defaults. Only transport addresses and independently measured artifact
// paths are normalized. All other arguments remain significant. Only a digest is
// returned; raw launch values must not be stored in diagnostic evidence. Known
// authentication environment keys and unrelated inherited variables are excluded.
func toolRuntimeConfiguration(args, environment []string, modelPath, projectorPath string, port int, templateSHA string) (string, error) {
	if len(args) < 2 || environment == nil || modelPath == "" || templateSHA == "" {
		return "", errors.New("owned runtime launch configuration is incomplete")
	}
	selected := append([]string(nil), args[1:]...)
	seen := map[string]bool{}
	for i := 0; i < len(selected); i++ {
		kind := selected[i]
		switch kind {
		case "-m", "--model":
			kind = "model"
		case "--port":
			kind = "port"
		case "--mmproj":
			kind = "projector"
		case "--chat-template-file":
			kind = "template"
		default:
			continue
		}
		if seen[kind] || i+1 >= len(selected) {
			return "", errors.New("ambiguous owned runtime launch configuration")
		}
		seen[kind] = true
		i++
		switch kind {
		case "model":
			if selected[i] != modelPath {
				return "", errors.New("owned runtime model path changed")
			}
			selected[i] = "@measured-model"
		case "port":
			if selected[i] != strconv.Itoa(port) {
				return "", errors.New("owned runtime endpoint changed")
			}
			selected[i] = "@owned-loopback-port"
		case "projector":
			if projectorPath == "" || selected[i] != projectorPath {
				return "", errors.New("owned runtime projector path changed")
			}
			selected[i] = "@measured-projector"
		case "template":
			selected[i] = "@observed-template:" + templateSHA
		}
	}
	if !seen["model"] || !seen["port"] || (projectorPath != "" && !seen["projector"]) {
		return "", errors.New("owned runtime artifact or endpoint binding is missing")
	}
	var measuredEnv []string
	seenEnv := map[string]bool{}
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		if key == "LLAMA_API_KEY" || key == "LLAMA_ARG_API_KEY" {
			continue
		}
		relevant := strings.HasPrefix(key, "GGML_") || strings.HasPrefix(key, "LLAMA_")
		switch key {
		case "CUDA_VISIBLE_DEVICES", "HIP_VISIBLE_DEVICES", "ROCR_VISIBLE_DEVICES", "GPU_DEVICE_ORDINAL", "OMP_NUM_THREADS", "PATH", "LD_LIBRARY_PATH", "LD_PRELOAD", "DYLD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES":
			relevant = true
		}
		if !relevant {
			continue
		}
		if seenEnv[key] {
			return "", errors.New("ambiguous owned runtime environment")
		}
		seenEnv[key] = true
		measuredEnv = append(measuredEnv, key+"="+value)
	}
	sort.Strings(measuredEnv)
	encoded, err := json.Marshal(struct {
		Version     int      `json:"version"`
		Platform    string   `json:"platform"`
		Arguments   []string `json:"arguments"`
		Environment []string `json:"environment"`
	}{1, runtime.GOOS + "/" + runtime.GOARCH, selected, measuredEnv})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
