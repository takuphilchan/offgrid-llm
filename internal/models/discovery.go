package models

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

type HFGated bool

func (g *HFGated) UnmarshalJSON(data []byte) error {
	var flag bool
	if err := json.Unmarshal(data, &flag); err == nil {
		*g = HFGated(flag)
		return nil
	}
	var mode string
	if err := json.Unmarshal(data, &mode); err != nil {
		return err
	}
	*g = HFGated(mode != "" && mode != "false")
	return nil
}

var hubRepository = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,95}/[A-Za-z0-9][A-Za-z0-9_.-]{0,95}$`)
var splitGGUF = regexp.MustCompile(`(?i)-\d{5}-of-\d{5}\.gguf$`)
var localStem = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func ValidHubRepository(repo string) bool {
	return hubRepository.MatchString(repo) && !strings.Contains(repo, "..")
}

func standaloneFileReason(filename string) string {
	name := strings.ToLower(filename)
	if splitGGUF.MatchString(name) {
		return "split_weights"
	}
	if strings.Contains(name, "mmproj") || strings.Contains(name, "projector") {
		return "projector"
	}
	return ""
}

type DiscoveredFile struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	Size      int64  `json:"size_bytes"`
	Quant     string `json:"quant"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
}

// File bytes are not a promise of runtime RAM or architecture compatibility.
// Split weights/projectors need coordinated assets and cannot be installed as
// standalone chat models. Show them with an explicit reason, not a false success.
func (hf *HuggingFaceClient) DiscoverFiles(ctx context.Context, repo string) ([]DiscoveredFile, error) {
	files, err := hf.GetModelFilesContext(ctx, repo)
	if err != nil {
		return nil, err
	}
	result := make([]DiscoveredFile, 0)
	for _, file := range files {
		name := strings.ToLower(file.Filename)
		if !strings.HasSuffix(name, ".gguf") {
			continue
		}
		stem := localStem.ReplaceAllString(strings.TrimSuffix(path.Base(file.Filename), path.Ext(file.Filename)), "-")
		if len(stem) > 80 {
			stem = stem[:80]
		}
		digest := sha256.Sum256([]byte(repo + "/" + file.Filename))
		item := DiscoveredFile{ID: fmt.Sprintf("%s-%x", stem, digest[:6]), File: file.Filename, Size: file.Size, Quant: extractQuantization(file.Filename), Supported: true}
		item.Reason = standaloneFileReason(name)
		item.Supported = item.Reason == ""
		result = append(result, item)
	}
	return result, nil
}
