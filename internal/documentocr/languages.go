package documentocr

import (
	_ "embed"
	"encoding/json"
	"slices"
)

// The service and isolated worker share this pinned catalog. Pack availability
// is not a recognition-quality claim. There is deliberately no network loader.
//
//go:embed languages.json
var languageManifest []byte

type LanguagePack struct {
	Language         string   `json:"language"`
	Name             string   `json:"name"`
	InterfaceLocales []string `json:"interface_locales"`
	SHA256           string   `json:"sha256"`
	Bytes            int      `json:"bytes"`
	SourceURL        string   `json:"source_url"`
	Qualification    string   `json:"qualification"`
}

type LanguageManifest struct {
	Version                     int            `json:"version"`
	SourceRevision              string         `json:"source_revision"`
	License                     string         `json:"license"`
	Packs                       []LanguagePack `json:"packs"`
	UnavailableInterfaceLocales []string       `json:"unavailable_interface_locales"`
}

func Languages() LanguageManifest {
	var result LanguageManifest
	// Embedded, version-controlled data; its schema and every pin are tested.
	if err := json.Unmarshal(languageManifest, &result); err != nil {
		panic("invalid OCR language manifest")
	}
	return result
}

func LookupLanguage(language string) (LanguagePack, bool) {
	for _, pack := range Languages().Packs {
		if pack.Language == language {
			return pack, true
		}
	}
	return LanguagePack{}, false
}

// Empty keeps legacy English imports compatible. Explicit choices never fall
// back to English, a UI locale, or a linguistically related language.
func NormalizeLanguage(language string) (string, bool) {
	if language == "" {
		language = "eng"
	}
	_, ok := LookupLanguage(language)
	return language, ok
}

func LanguagesForLocale(locale string) []string {
	result := []string{}
	for _, pack := range Languages().Packs {
		if slices.Contains(pack.InterfaceLocales, locale) {
			result = append(result, pack.Language)
		}
	}
	return result
}
