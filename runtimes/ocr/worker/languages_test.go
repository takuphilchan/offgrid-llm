package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func TestPinnedLanguageInitialization(t *testing.T) {
	dir := os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if dir == "" {
		t.Skip("requires explicitly obtained pinned packs")
	}
	for _, pack := range documentocr.Languages().Packs {
		t.Run(pack.Language, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			data, err := os.ReadFile(filepath.Join(dir, pack.Language+".traineddata"))
			if err != nil {
				t.Fatal(err)
			}
			if digest(data) != pack.SHA256 {
				t.Fatal("wrong fixture")
			}
			engine, err := newEngineForLanguage(ctx, data, pack.Language)
			if err != nil {
				t.Fatal(err)
			}
			engine.close()
		})
	}
}

func TestLanguageReceiptRejectsCrossPackAndUnknownSelection(t *testing.T) {
	for _, language := range []string{"eng+eng", "sn", "../eng", ""} {
		b, _ := json.Marshal(request{Protocol: 1, Language: language})
		if got := process(context.Background(), b); got.Error != "document_ocr_language_unavailable" {
			t.Fatal(language, got)
		}
	}
	b, _ := json.Marshal(request{Protocol: 1, Language: "chi_sim", LanguageSHA: languageSHA, Model: []byte("wrong pack")})
	if got := process(context.Background(), b); got.Error != "document_ocr_language_invalid" {
		t.Fatal(got)
	}
}

func TestPinnedMixedLanguageEngineLoadsEverySelectedPack(t *testing.T) {
	dir := os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if dir == "" {
		t.Skip("requires explicitly obtained pinned packs")
	}
	for _, selection := range []string{"chi_sim+fra+eng", "ara+eng+fra"} {
		t.Run(selection, func(t *testing.T) {
			models := [][]byte{}
			for _, id := range strings.Split(selection, "+") {
				data, err := os.ReadFile(filepath.Join(dir, id+".traineddata"))
				if err != nil {
					t.Fatal(err)
				}
				models = append(models, data)
			}
			bundle, err := documentocr.Models(selection, models)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			engine, err := newEngineForLanguage(ctx, bundle, selection)
			if err != nil {
				t.Fatal(err)
			}
			engine.close()
			// Bypass Go's digest admission to exercise the bridge's separate
			// refusal of Tesseract success after loading only a subset.
			clear(bundle[len(bundle)-len(models[len(models)-1]):])
			engine, err = newEngineForLanguage(ctx, bundle, selection)
			if err == nil {
				engine.close()
				t.Fatal("bridge accepted only part of the requested selection")
			}
		})
	}
}
