package main

import "testing"

func TestIsLikelyEmbeddingModel(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "bge-m3-Q4_K_M", want: true},
		{name: "/models/bge-m3-Q4_K_M.gguf", want: true},
		{name: "all-MiniLM-L6-v2", want: true},
		{name: "nomic-embed-text-v1.5.Q4_K_M.gguf", want: true},
		{name: "Llama-3.2-3B-Instruct-Q4_K_M.gguf", want: false},
		{name: "qwen2.5-3b-instruct", want: false},
		{name: "mistral-7b-instruct", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLikelyEmbeddingModel(tt.name); got != tt.want {
				t.Fatalf("isLikelyEmbeddingModel(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
