package documentocr

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageManifestPinsAndLocaleBoundaries(t *testing.T) {
	m := Languages()
	if m.Version != 1 || len(m.SourceRevision) != 40 || len(m.Packs) != 25 {
		t.Fatal("invalid manifest", m)
	}
	seen := map[string]bool{}
	for _, p := range m.Packs {
		_, err := hex.DecodeString(p.SHA256)
		if seen[p.Language] || p.Language == "" || err != nil || len(p.SHA256) != 64 || p.Bytes <= 0 || p.Bytes > 32<<20 || p.Qualification != "unverified" || p.SourceURL != "https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/"+m.SourceRevision+"/"+p.Language+".traineddata" {
			t.Fatal("invalid pin", p)
		}
		seen[p.Language] = true
	}
	for _, locale := range []string{"sn", "nd", "zu", "zh", "xx", "en-US"} {
		if len(LanguagesForLocale(locale)) != 0 {
			t.Fatal("implicit substitution", locale)
		}
	}
	for locale, want := range map[string]string{"zh-Hans": "chi_sim", "zh-Hant": "chi_tra", "pt-BR": "por", "pt-PT": "por", "sw": "swa", "ar": "ara", "ur": "urd"} {
		got := LanguagesForLocale(locale)
		if len(got) != 1 || got[0] != want {
			t.Fatal(locale, got)
		}
	}
	for _, language := range []string{"../eng", "ENG", "eng+chi_sim", "sn", "eng\x00"} {
		if _, ok := LookupLanguage(language); ok {
			t.Fatal("unsupported selection", language)
		}
	}
	// Callers cannot mutate the embedded catalog for subsequent imports.
	m.Packs[0].SHA256 = strings.Repeat("0", 64)
	if p, _ := LookupLanguage("eng"); p.SHA256 == m.Packs[0].SHA256 {
		t.Fatal("mutable catalog")
	}
}

func TestActualPinnedLanguageFiles(t *testing.T) {
	dir := os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if dir == "" {
		t.Skip("requires explicitly obtained pinned packs")
	}
	for _, pack := range Languages().Packs {
		t.Run(pack.Language, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, pack.Language+".traineddata"))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			if len(data) != pack.Bytes || hex.EncodeToString(sum[:]) != pack.SHA256 {
				t.Fatal("pack pin mismatch")
			}
		})
	}
}
