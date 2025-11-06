package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HuggingFaceFileInfo represents file metadata from HF API
type HuggingFaceFileInfo struct {
	Type string `json:"type"`
	Oid  string `json:"oid"`
	Size int64  `json:"size"`
	Path string `json:"path"`
	LFS  *struct {
		Oid         string `json:"oid"`
		Size        int64  `json:"size"`
		PointerSize int    `json:"pointerSize"`
	} `json:"lfs,omitempty"`
}

func main() {
	// Models to fetch hashes for
	repos := []string{
		"TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF",
		"TheBloke/Llama-2-7B-Chat-GGUF",
		"TheBloke/Mistral-7B-Instruct-v0.2-GGUF",
		"TheBloke/phi-2-GGUF",
	}

	for _, repo := range repos {
		fmt.Printf("\n=== %s ===\n", repo)
		fetchRepoHashes(repo)
	}
}

func fetchRepoHashes(repo string) {
	// Fetch file tree from HuggingFace API
	url := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", repo)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching %s: %v\n", repo, err)
		return
	}
	defer resp.Body.Close()

	var files []HuggingFaceFileInfo
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		fmt.Printf("Error decoding response: %v\n", err)
		return
	}

	// Print info for .gguf files
	for _, file := range files {
		if len(file.Path) > 5 && file.Path[len(file.Path)-5:] == ".gguf" {
			hash := file.Oid
			if file.LFS != nil {
				hash = file.LFS.Oid
			}
			fmt.Printf("  %s\n", file.Path)
			fmt.Printf("    Size: %d bytes (%.2f GB)\n", file.Size, float64(file.Size)/1024/1024/1024)
			fmt.Printf("    SHA256: %s\n", hash)
			fmt.Println()
		}
	}
}
