package models

import (
	"testing"
)

func TestCatalog(t *testing.T) {
	catalog := DefaultCatalog()

	t.Run("catalog has models", func(t *testing.T) {
		if len(catalog.Models) == 0 {
			t.Fatal("catalog should have models")
		}

		if len(catalog.Models) != 4 {
			t.Errorf("expected 4 models, got %d", len(catalog.Models))
		}
	})

	t.Run("find tinyllama", func(t *testing.T) {
		model := catalog.FindModel("tinyllama-1.1b-chat")
		if model == nil {
			t.Fatal("TinyLlama not found in catalog")
		}

		if model.Name != "TinyLlama 1.1B Chat" {
			t.Errorf("unexpected model name: %s", model.Name)
		}

		if model.Parameters != "1.1B" {
			t.Errorf("unexpected parameters: %s", model.Parameters)
		}
	})

	t.Run("find llama-2", func(t *testing.T) {
		model := catalog.FindModel("llama-2-7b-chat")
		if model == nil {
			t.Fatal("Llama 2 not found in catalog")
		}

		if model.Parameters != "7B" {
			t.Errorf("unexpected parameters: %s", model.Parameters)
		}
	})

	t.Run("find mistral", func(t *testing.T) {
		model := catalog.FindModel("mistral-7b-instruct")
		if model == nil {
			t.Fatal("Mistral not found in catalog")
		}

		if !model.Recommended {
			t.Error("Mistral should be recommended")
		}
	})

	t.Run("find phi-2", func(t *testing.T) {
		model := catalog.FindModel("phi-2")
		if model == nil {
			t.Fatal("Phi-2 not found in catalog")
		}

		if model.Provider != "Microsoft" {
			t.Errorf("unexpected provider: %s", model.Provider)
		}
	})

	t.Run("case insensitive search", func(t *testing.T) {
		model1 := catalog.FindModel("TinyLlama-1.1B-Chat")
		model2 := catalog.FindModel("tinyllama-1.1b-chat")
		model3 := catalog.FindModel("TINYLLAMA-1.1B-CHAT")

		if model1 == nil || model2 == nil || model3 == nil {
			t.Fatal("case-insensitive search failed")
		}

		if model1.ID != model2.ID || model2.ID != model3.ID {
			t.Error("case-insensitive search returned different models")
		}
	})

	t.Run("nonexistent model", func(t *testing.T) {
		model := catalog.FindModel("nonexistent-model")
		if model != nil {
			t.Error("should return nil for nonexistent model")
		}
	})
}

func TestModelVariants(t *testing.T) {
	catalog := DefaultCatalog()

	t.Run("tinyllama variants", func(t *testing.T) {
		model := catalog.FindModel("tinyllama-1.1b-chat")
		if model == nil {
			t.Fatal("TinyLlama not found")
		}

		if len(model.Variants) < 2 {
			t.Errorf("expected at least 2 variants, got %d", len(model.Variants))
		}

		// Check Q4_K_M variant exists
		q4 := model.FindVariant("Q4_K_M")
		if q4 == nil {
			t.Fatal("Q4_K_M variant not found")
		}

		if q4.Quality != "high" {
			t.Errorf("unexpected quality: %s", q4.Quality)
		}

		if len(q4.Sources) == 0 {
			t.Error("variant should have sources")
		}
	})

	t.Run("get best variant", func(t *testing.T) {
		model := catalog.FindModel("tinyllama-1.1b-chat")
		if model == nil {
			t.Fatal("TinyLlama not found")
		}

		// With 16GB RAM, should get highest quality
		best := model.GetBestVariant(16)
		if best == nil {
			t.Fatal("should return a variant for 16GB RAM")
		}

		// With 1GB RAM, should get smallest variant
		small := model.GetBestVariant(1)
		if small != nil {
			if small.Size > best.Size {
				t.Error("small RAM should return smaller variant")
			}
		}
	})

	t.Run("catalog GetBestVariant", func(t *testing.T) {
		variant := catalog.GetBestVariant("tinyllama-1.1b-chat")
		if variant == nil {
			t.Fatal("GetBestVariant should return a variant")
		}

		if variant.Quantization == "" {
			t.Error("variant should have quantization")
		}
	})
}

func TestDownloader(t *testing.T) {
	tmpDir := t.TempDir()
	catalog := DefaultCatalog()

	t.Run("create downloader", func(t *testing.T) {
		dm := NewDownloader(tmpDir, catalog)
		if dm == nil {
			t.Fatal("NewDownloader returned nil")
		}

		if dm.modelsDir != tmpDir {
			t.Errorf("models dir mismatch: got %s, want %s", dm.modelsDir, tmpDir)
		}

		if dm.catalog == nil {
			t.Error("catalog should not be nil")
		}

		if dm.client == nil {
			t.Error("HTTP client should not be nil")
		}
	})

	t.Run("set progress callback", func(t *testing.T) {
		dm := NewDownloader(tmpDir, catalog)

		dm.SetProgressCallback(func(p DownloadProgress) {
			// Callback function
		})

		// Callback should be set
		if dm.onProgress == nil {
			t.Error("progress callback was not set")
		}
	})
}
