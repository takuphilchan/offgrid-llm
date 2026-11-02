package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParserOCRVisibleFixturePins(t *testing.T) {
	data, err := os.ReadFile("testdata/documents/ocr/inputs/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases []struct {
			Path     string `json:"path"`
			SHA256   string `json:"sha256"`
			Rotation int    `json:"rotation"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || len(manifest.Cases) != 6 {
		t.Fatal("invalid visible OCR corpus", err)
	}
	seen := map[string]bool{}
	for _, row := range manifest.Cases {
		if row.Path != filepath.Base(row.Path) || seen[row.Path] {
			t.Fatal("invalid fixture identity", row.Path)
		}
		seen[row.Path] = true
		data, err := os.ReadFile(filepath.Join("testdata/documents/ocr/inputs", row.Path))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != row.SHA256 {
			t.Fatalf("visible OCR fixture drift: %s", row.Path)
		}
	}
}

func TestParserOCRCorpusPins(t *testing.T) {
	data, err := os.ReadFile("testdata/documents/ocr/evaluation.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Results []struct {
			Fixture string `json:"fixture"`
			Source  string `json:"source_sha256"`
			Output  string `json:"output_sha256"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Results) != 3 {
		t.Fatal("evaluation corpus changed without review")
	}
	for _, row := range manifest.Results {
		for _, file := range []struct{ path, digest string }{
			{"testdata/documents/ocr/" + row.Fixture + ".hocr", row.Output},
			{"testdata/documents/corpus/" + row.Fixture + ".pdf", row.Source},
		} {
			data, err := os.ReadFile(file.path)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(data)
			if hex.EncodeToString(digest[:]) != file.digest {
				t.Fatalf("evaluation evidence drift: %s", file.path)
			}
		}
	}
}

func TestParserOCRActualEvaluationReceipts(t *testing.T) {
	for _, test := range []struct{ name, expected string }{
		{"scan", "Scanned receipt total 42"}, {"mixed", "Scanned page total 34"}, {"scan-rotated", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/documents/ocr/" + test.name + ".hocr")
			if err != nil {
				t.Fatal(err)
			}
			words, err := parseHOCR(string(data), 1700, 2200)
			if err != nil || len(words) == 0 {
				t.Fatalf("actual receipt rejected: %v", err)
			}
			var text []string
			low := false
			for _, word := range words {
				text = append(text, word.Text)
				low = low || word.Confidence < 50
			}
			if test.expected != "" && strings.Join(text, " ") != test.expected {
				t.Fatalf("actual word order changed: %q", text)
			}
			if test.name == "scan-rotated" && !low {
				t.Fatal("clipped scan uncertainty lost")
			}
		})
	}
}

const hocrFixture = `<?xml version="1.0"?><!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"><html><body><div class="ocr_page" title="bbox 0 0 100 100"><span class="ocr_line"><span class="ocrx_word" title="bbox 1 2 30 10; x_wconf 96">café</span><span class="ocrx_word" title="bbox 31 2 60 10; x_wconf 37">日本語</span></span><span class="ocr_line"><span class="ocrx_word" title="bbox 1 20 30 30; x_wconf 88">A&amp;B</span></span></div></body></html>`

func TestParserOCRPreservesWordsAndUncertainty(t *testing.T) {
	words, err := parseHOCR(hocrFixture, 100, 100)
	if err != nil || len(words) != 3 {
		t.Fatalf("decode: %+v %v", words, err)
	}
	if words[0].Text != "café" || words[1].Text != "日本語" || words[1].Confidence != 37 || words[2].Text != "A&B" || words[0].Line != 1 || words[2].Line != 2 || words[2].Bounds != [4]int{1, 20, 30, 30} {
		t.Fatalf("source fidelity lost: %+v", words)
	}
}

func TestParserOCRRejectsInvalidPage(t *testing.T) {
	for name, value := range map[string]string{
		"size":                 strings.Repeat("x", (2<<20)+1),
		"truncated":            hocrFixture[:len(hocrFixture)-10],
		"dimension":            strings.Replace(hocrFixture, "bbox 0 0 100 100", "bbox 0 0 101 100", 1),
		"negative":             strings.Replace(hocrFixture, "bbox 1 2 30 10", "bbox -1 2 30 10", 1),
		"inverted":             strings.Replace(hocrFixture, "bbox 1 2 30 10", "bbox 31 2 30 10", 1),
		"outside":              strings.Replace(hocrFixture, "bbox 1 2 30 10", "bbox 1 2 101 10", 1),
		"missing-confidence":   strings.Replace(hocrFixture, "; x_wconf 96", "", 1),
		"high-confidence":      strings.Replace(hocrFixture, "x_wconf 96", "x_wconf 101", 1),
		"nan":                  strings.Replace(hocrFixture, "x_wconf 96", "x_wconf NaN", 1),
		"duplicate-confidence": strings.Replace(hocrFixture, "x_wconf 96", "x_wconf 96; x_wconf 100", 1),
		"duplicate-box":        strings.Replace(hocrFixture, "x_wconf 96", "x_wconf 96; bbox 1 2 30 10", 1),
		"entity":               strings.Replace(hocrFixture, "café", "&external;", 1),
		"empty-word":           strings.Replace(hocrFixture, "café", " ", 1),
		"missing-line":         strings.ReplaceAll(hocrFixture, `class="ocr_line"`, `class="not-line"`),
		"missing-page":         strings.Replace(hocrFixture, `class="ocr_page"`, `class="not-page"`, 1),
		"second-page":          hocrFixture + hocrFixture,
		"second-root":          hocrFixture + "<extra/>",
		"trailing-text":        hocrFixture + "unbound text",
		"invalid-utf8":         strings.Replace(hocrFixture, "café", string([]byte{0xff}), 1),
		"text-limit":           strings.Replace(hocrFixture, "café", strings.Repeat("a", (1<<20)+1), 1),
		"depth":                strings.Repeat("<a>", 65) + hocrFixture + strings.Repeat("</a>", 65),
	} {
		t.Run(name, func(t *testing.T) {
			if words, err := parseHOCR(value, 100, 100); err == nil || words != nil {
				t.Fatal("accepted invalid OCR content")
			}
		})
	}
}
