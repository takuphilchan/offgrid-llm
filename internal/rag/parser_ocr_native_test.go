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

// These are captured engine receipts, not a substitute for running the packaged
// OCR worker. Preserve both successful content and the current rotation failures.
func TestParserOCRMaintainedNativeReceipts(t *testing.T) {
	base := "testdata/documents"
	data, err := os.ReadFile(filepath.Join(base, "ocr/native/evaluation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Results []struct {
			Fixture   string `json:"fixture"`
			Source    string `json:"source"`
			SourceSHA string `json:"source_sha256"`
			OutputSHA string `json:"output_sha256"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Expected  string `json:"expected"`
			Actual    string `json:"actual"`
			Exact     bool   `json:"exact_match"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || len(manifest.Results) != 7 {
		t.Fatal("invalid native evaluation inventory", err)
	}
	passed := 0
	for _, row := range manifest.Results {
		t.Run(row.Fixture, func(t *testing.T) {
			receipt, err := os.ReadFile(filepath.Join(base, "ocr/native", row.Fixture+".hocr"))
			if err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile(filepath.Join(base, row.Source))
			if err != nil {
				t.Fatal(err)
			}
			for _, pair := range []struct {
				data []byte
				hash string
			}{{source, row.SourceSHA}, {receipt, row.OutputSHA}} {
				digest := sha256.Sum256(pair.data)
				if hex.EncodeToString(digest[:]) != pair.hash {
					t.Fatal("native evidence digest changed")
				}
			}
			words, err := parseHOCR(string(receipt), row.Width, row.Height)
			if err != nil {
				t.Fatal(err)
			}
			var text []string
			low := false
			for _, word := range words {
				text = append(text, word.Text)
				low = low || word.Confidence < 50
			}
			actual := strings.Join(text, " ")
			if actual != row.Actual || (actual == row.Expected) != row.Exact {
				t.Fatal("recognition evidence changed")
			}
			if row.Exact {
				passed++
			} else if !low {
				t.Fatal("failed rotation uncertainty lost")
			}
		})
	}
	if passed != 5 {
		t.Fatal("native evaluation score changed without new evidence", passed)
	}
}
