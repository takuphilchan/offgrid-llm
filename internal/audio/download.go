package audio

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// WhisperModelInfo contains whisper model download info
type WhisperModelInfo struct {
	Name string
	Size string
	URL  string
}

// PiperVoiceInfo contains piper voice download info
type PiperVoiceInfo struct {
	Name     string
	Language string
	Quality  string
	URL      string
	JSONUrl  string
}

// Available whisper models
var WhisperModels = map[string]WhisperModelInfo{
	"tiny.en": {
		Name: "tiny.en",
		Size: "75 MB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin",
	},
	"tiny": {
		Name: "tiny",
		Size: "75 MB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
	},
	"base.en": {
		Name: "base.en",
		Size: "142 MB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin",
	},
	"base": {
		Name: "base",
		Size: "142 MB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
	},
	"small.en": {
		Name: "small.en",
		Size: "466 MB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin",
	},
	"small": {
		Name: "small",
		Size: "466 MB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
	},
	"medium.en": {
		Name: "medium.en",
		Size: "1.5 GB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.en.bin",
	},
	"medium": {
		Name: "medium",
		Size: "1.5 GB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
	},
	"large-v3": {
		Name: "large-v3",
		Size: "3.1 GB",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin",
	},
}

// Piper voices - comprehensive list of languages
// See: https://huggingface.co/rhasspy/piper-voices/tree/main
var PiperVoices = map[string]PiperVoiceInfo{
	// English - US
	"en_US-amy-medium": {
		Name:     "en_US-amy-medium",
		Language: "English (US)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium/en_US-amy-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium/en_US-amy-medium.onnx.json",
	},
	"en_US-lessac-medium": {
		Name:     "en_US-lessac-medium",
		Language: "English (US)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json",
	},
	"en_US-libritts-high": {
		Name:     "en_US-libritts-high",
		Language: "English (US)",
		Quality:  "high",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/libritts/high/en_US-libritts-high.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/libritts/high/en_US-libritts-high.onnx.json",
	},
	"en_US-ryan-medium": {
		Name:     "en_US-ryan-medium",
		Language: "English (US)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/ryan/medium/en_US-ryan-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/ryan/medium/en_US-ryan-medium.onnx.json",
	},
	// English - UK
	"en_GB-alan-medium": {
		Name:     "en_GB-alan-medium",
		Language: "English (UK)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_GB/alan/medium/en_GB-alan-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_GB/alan/medium/en_GB-alan-medium.onnx.json",
	},
	"en_GB-alba-medium": {
		Name:     "en_GB-alba-medium",
		Language: "English (UK)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_GB/alba/medium/en_GB-alba-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_GB/alba/medium/en_GB-alba-medium.onnx.json",
	},
	// Spanish
	"es_ES-davefx-medium": {
		Name:     "es_ES-davefx-medium",
		Language: "Spanish (Spain)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json",
	},
	"es_ES-sharvard-medium": {
		Name:     "es_ES-sharvard-medium",
		Language: "Spanish (Spain)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/sharvard/medium/es_ES-sharvard-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/sharvard/medium/es_ES-sharvard-medium.onnx.json",
	},
	"es_MX-ald-medium": {
		Name:     "es_MX-ald-medium",
		Language: "Spanish (Mexico)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_MX/ald/medium/es_MX-ald-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_MX/ald/medium/es_MX-ald-medium.onnx.json",
	},
	// French
	"fr_FR-siwis-medium": {
		Name:     "fr_FR-siwis-medium",
		Language: "French",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/fr/fr_FR/siwis/medium/fr_FR-siwis-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/fr/fr_FR/siwis/medium/fr_FR-siwis-medium.onnx.json",
	},
	"fr_FR-upmc-medium": {
		Name:     "fr_FR-upmc-medium",
		Language: "French",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/fr/fr_FR/upmc/medium/fr_FR-upmc-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/fr/fr_FR/upmc/medium/fr_FR-upmc-medium.onnx.json",
	},
	// German
	"de_DE-thorsten-medium": {
		Name:     "de_DE-thorsten-medium",
		Language: "German",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx.json",
	},
	"de_DE-thorsten_emotional-medium": {
		Name:     "de_DE-thorsten_emotional-medium",
		Language: "German",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten_emotional/medium/de_DE-thorsten_emotional-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten_emotional/medium/de_DE-thorsten_emotional-medium.onnx.json",
	},
	// Italian
	"it_IT-riccardo-x_low": {
		Name:     "it_IT-riccardo-x_low",
		Language: "Italian",
		Quality:  "x_low",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx.json",
	},
	// Portuguese
	"pt_BR-faber-medium": {
		Name:     "pt_BR-faber-medium",
		Language: "Portuguese (Brazil)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/pt/pt_BR/faber/medium/pt_BR-faber-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/pt/pt_BR/faber/medium/pt_BR-faber-medium.onnx.json",
	},
	"pt_PT-tugao-medium": {
		Name:     "pt_PT-tugao-medium",
		Language: "Portuguese (Portugal)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/pt/pt_PT/tugao/medium/pt_PT-tugao-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/pt/pt_PT/tugao/medium/pt_PT-tugao-medium.onnx.json",
	},
	// Russian
	"ru_RU-irina-medium": {
		Name:     "ru_RU-irina-medium",
		Language: "Russian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ru/ru_RU/irina/medium/ru_RU-irina-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ru/ru_RU/irina/medium/ru_RU-irina-medium.onnx.json",
	},
	"ru_RU-ruslan-medium": {
		Name:     "ru_RU-ruslan-medium",
		Language: "Russian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ru/ru_RU/ruslan/medium/ru_RU-ruslan-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ru/ru_RU/ruslan/medium/ru_RU-ruslan-medium.onnx.json",
	},
	// Chinese
	"zh_CN-huayan-medium": {
		Name:     "zh_CN-huayan-medium",
		Language: "Chinese (Mandarin)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/zh/zh_CN/huayan/medium/zh_CN-huayan-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/zh/zh_CN/huayan/medium/zh_CN-huayan-medium.onnx.json",
	},
	// Japanese
	"ja_JP-kokoro-medium": {
		Name:     "ja_JP-kokoro-medium",
		Language: "Japanese",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ja/ja_JP/kokoro/medium/ja_JP-kokoro-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ja/ja_JP/kokoro/medium/ja_JP-kokoro-medium.onnx.json",
	},
	// Korean
	"ko_KR-kss-x_low": {
		Name:     "ko_KR-kss-x_low",
		Language: "Korean",
		Quality:  "x_low",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ko/ko_KR/kss/x_low/ko_KR-kss-x_low.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ko/ko_KR/kss/x_low/ko_KR-kss-x_low.onnx.json",
	},
	// Dutch
	"nl_NL-mls-medium": {
		Name:     "nl_NL-mls-medium",
		Language: "Dutch",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/nl/nl_NL/mls/medium/nl_NL-mls-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/nl/nl_NL/mls/medium/nl_NL-mls-medium.onnx.json",
	},
	"nl_BE-nathalie-medium": {
		Name:     "nl_BE-nathalie-medium",
		Language: "Dutch (Belgium)",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/nl/nl_BE/nathalie/medium/nl_BE-nathalie-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/nl/nl_BE/nathalie/medium/nl_BE-nathalie-medium.onnx.json",
	},
	// Polish
	"pl_PL-darkman-medium": {
		Name:     "pl_PL-darkman-medium",
		Language: "Polish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/pl/pl_PL/darkman/medium/pl_PL-darkman-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/pl/pl_PL/darkman/medium/pl_PL-darkman-medium.onnx.json",
	},
	"pl_PL-gosia-medium": {
		Name:     "pl_PL-gosia-medium",
		Language: "Polish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/pl/pl_PL/gosia/medium/pl_PL-gosia-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/pl/pl_PL/gosia/medium/pl_PL-gosia-medium.onnx.json",
	},
	// Arabic
	"ar_JO-kareem-medium": {
		Name:     "ar_JO-kareem-medium",
		Language: "Arabic",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ar/ar_JO/kareem/medium/ar_JO-kareem-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ar/ar_JO/kareem/medium/ar_JO-kareem-medium.onnx.json",
	},
	// Turkish
	"tr_TR-dfki-medium": {
		Name:     "tr_TR-dfki-medium",
		Language: "Turkish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/tr/tr_TR/dfki/medium/tr_TR-dfki-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/tr/tr_TR/dfki/medium/tr_TR-dfki-medium.onnx.json",
	},
	"tr_TR-fettah-medium": {
		Name:     "tr_TR-fettah-medium",
		Language: "Turkish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/tr/tr_TR/fettah/medium/tr_TR-fettah-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/tr/tr_TR/fettah/medium/tr_TR-fettah-medium.onnx.json",
	},
	// Vietnamese
	"vi_VN-vivos-x_low": {
		Name:     "vi_VN-vivos-x_low",
		Language: "Vietnamese",
		Quality:  "x_low",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/vi/vi_VN/vivos/x_low/vi_VN-vivos-x_low.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/vi/vi_VN/vivos/x_low/vi_VN-vivos-x_low.onnx.json",
	},
	// Hindi
	"hi_IN-cmu_indic-medium": {
		Name:     "hi_IN-cmu_indic-medium",
		Language: "Hindi",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/hi/hi_IN/cmu_indic/medium/hi_IN-cmu_indic-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/hi/hi_IN/cmu_indic/medium/hi_IN-cmu_indic-medium.onnx.json",
	},
	// Greek
	"el_GR-rapunzelina-low": {
		Name:     "el_GR-rapunzelina-low",
		Language: "Greek",
		Quality:  "low",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/el/el_GR/rapunzelina/low/el_GR-rapunzelina-low.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/el/el_GR/rapunzelina/low/el_GR-rapunzelina-low.onnx.json",
	},
	// Czech
	"cs_CZ-jirka-medium": {
		Name:     "cs_CZ-jirka-medium",
		Language: "Czech",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/cs/cs_CZ/jirka/medium/cs_CZ-jirka-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/cs/cs_CZ/jirka/medium/cs_CZ-jirka-medium.onnx.json",
	},
	// Hungarian
	"hu_HU-anna-medium": {
		Name:     "hu_HU-anna-medium",
		Language: "Hungarian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/hu/hu_HU/anna/medium/hu_HU-anna-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/hu/hu_HU/anna/medium/hu_HU-anna-medium.onnx.json",
	},
	// Romanian
	"ro_RO-mihai-medium": {
		Name:     "ro_RO-mihai-medium",
		Language: "Romanian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ro/ro_RO/mihai/medium/ro_RO-mihai-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ro/ro_RO/mihai/medium/ro_RO-mihai-medium.onnx.json",
	},
	// Swedish
	"sv_SE-nst-medium": {
		Name:     "sv_SE-nst-medium",
		Language: "Swedish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/sv/sv_SE/nst/medium/sv_SE-nst-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/sv/sv_SE/nst/medium/sv_SE-nst-medium.onnx.json",
	},
	// Norwegian
	"no_NO-talesyntese-medium": {
		Name:     "no_NO-talesyntese-medium",
		Language: "Norwegian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/no/no_NO/talesyntese/medium/no_NO-talesyntese-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/no/no_NO/talesyntese/medium/no_NO-talesyntese-medium.onnx.json",
	},
	// Danish
	"da_DK-talesyntese-medium": {
		Name:     "da_DK-talesyntese-medium",
		Language: "Danish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/da/da_DK/talesyntese/medium/da_DK-talesyntese-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/da/da_DK/talesyntese/medium/da_DK-talesyntese-medium.onnx.json",
	},
	// Finnish
	"fi_FI-harri-medium": {
		Name:     "fi_FI-harri-medium",
		Language: "Finnish",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/fi/fi_FI/harri/medium/fi_FI-harri-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/fi/fi_FI/harri/medium/fi_FI-harri-medium.onnx.json",
	},
	// Ukrainian
	"uk_UA-lada-x_low": {
		Name:     "uk_UA-lada-x_low",
		Language: "Ukrainian",
		Quality:  "x_low",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/uk/uk_UA/lada/x_low/uk_UA-lada-x_low.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/uk/uk_UA/lada/x_low/uk_UA-lada-x_low.onnx.json",
	},
	// Catalan
	"ca_ES-upc_ona-medium": {
		Name:     "ca_ES-upc_ona-medium",
		Language: "Catalan",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ca/ca_ES/upc_ona/medium/ca_ES-upc_ona-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ca/ca_ES/upc_ona/medium/ca_ES-upc_ona-medium.onnx.json",
	},
	// Icelandic
	"is_IS-ugla-medium": {
		Name:     "is_IS-ugla-medium",
		Language: "Icelandic",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/is/is_IS/ugla/medium/is_IS-ugla-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/is/is_IS/ugla/medium/is_IS-ugla-medium.onnx.json",
	},
	// Swahili
	"sw_CD-lanfrica-medium": {
		Name:     "sw_CD-lanfrica-medium",
		Language: "Swahili",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/sw/sw_CD/lanfrica/medium/sw_CD-lanfrica-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/sw/sw_CD/lanfrica/medium/sw_CD-lanfrica-medium.onnx.json",
	},
	// Nepali
	"ne_NP-google-medium": {
		Name:     "ne_NP-google-medium",
		Language: "Nepali",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/ne/ne_NP/google/medium/ne_NP-google-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/ne/ne_NP/google/medium/ne_NP-google-medium.onnx.json",
	},
	// Serbian
	"sr_RS-serbski_institut-medium": {
		Name:     "sr_RS-serbski_institut-medium",
		Language: "Serbian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/sr/sr_RS/serbski_institut/medium/sr_RS-serbski_institut-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/sr/sr_RS/serbski_institut/medium/sr_RS-serbski_institut-medium.onnx.json",
	},
	// Kazakh
	"kk_KZ-iseke-x_low": {
		Name:     "kk_KZ-iseke-x_low",
		Language: "Kazakh",
		Quality:  "x_low",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/kk/kk_KZ/iseke/x_low/kk_KZ-iseke-x_low.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/kk/kk_KZ/iseke/x_low/kk_KZ-iseke-x_low.onnx.json",
	},
	// Slovenian
	"sl_SI-artur-medium": {
		Name:     "sl_SI-artur-medium",
		Language: "Slovenian",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/sl/sl_SI/artur/medium/sl_SI-artur-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/sl/sl_SI/artur/medium/sl_SI-artur-medium.onnx.json",
	},
	// Slovak
	"sk_SK-lili-medium": {
		Name:     "sk_SK-lili-medium",
		Language: "Slovak",
		Quality:  "medium",
		URL:      "https://huggingface.co/rhasspy/piper-voices/resolve/main/sk/sk_SK/lili/medium/sk_SK-lili-medium.onnx",
		JSONUrl:  "https://huggingface.co/rhasspy/piper-voices/resolve/main/sk/sk_SK/lili/medium/sk_SK-lili-medium.onnx.json",
	},
}

// DownloadProgressFunc is called during download with progress info
type DownloadProgressFunc func(downloaded, total int64)

// defaultProgressFunc creates a default progress printer
func defaultProgressFunc(name string) DownloadProgressFunc {
	var lastPercent int64 = -1
	return func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		percent := (downloaded * 100) / total
		if percent != lastPercent {
			lastPercent = percent
			// Progress bar
			barWidth := 40
			filled := int(percent) * barWidth / 100
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			fmt.Printf("\r  %s [%s] %3d%% (%.1f/%.1f MB)", name, bar, percent,
				float64(downloaded)/(1024*1024), float64(total)/(1024*1024))
			if percent >= 100 {
				fmt.Println()
			}
		}
	}
}

// DownloadWhisperModel downloads a whisper model
func (e *Engine) DownloadWhisperModel(name string, progress DownloadProgressFunc) error {
	model, ok := WhisperModels[name]
	if !ok {
		return fmt.Errorf("unknown whisper model: %s", name)
	}

	if progress == nil {
		progress = defaultProgressFunc(name)
	}

	destPath := filepath.Join(e.whisperDir, "ggml-"+name+".bin")
	return downloadFile(model.URL, destPath, progress)
}

// DownloadPiperVoice downloads a piper voice
func (e *Engine) DownloadPiperVoice(name string, progress DownloadProgressFunc) error {
	voice, ok := PiperVoices[name]
	if !ok {
		return fmt.Errorf("unknown piper voice: %s", name)
	}

	if progress == nil {
		progress = defaultProgressFunc(name)
	}

	// Download ONNX model
	destPath := filepath.Join(e.piperDir, name+".onnx")
	if err := downloadFile(voice.URL, destPath, progress); err != nil {
		return err
	}

	// Download JSON config
	jsonPath := filepath.Join(e.piperDir, name+".onnx.json")
	if err := downloadFile(voice.JSONUrl, jsonPath, nil); err != nil {
		// JSON is optional, don't fail
		fmt.Printf("Warning: could not download voice config: %v\n", err)
	}

	return nil
}

// DownloadWhisperBinary downloads whisper.cpp binary if not already installed
// The binary is normally bundled with the OffGrid installation, so this is a fallback
func (e *Engine) DownloadWhisperBinary(progress DownloadProgressFunc) error {
	// Check if already installed (from bundle or previous download)
	if e.whisperPath != "" {
		fmt.Println("Whisper.cpp is already installed")
		return nil
	}

	// Re-check for existing binary
	foundPath := e.findWhisper()
	if foundPath != "" {
		e.whisperPath = foundPath
		fmt.Printf("Found existing whisper.cpp at %s\n", foundPath)
		return nil
	}

	// Not installed - build from source
	return e.buildWhisperFromSource(progress)
}

// buildWhisperFromSource clones and builds whisper.cpp from source
func (e *Engine) buildWhisperFromSource(progress DownloadProgressFunc) error {
	fmt.Println("Building whisper.cpp from source...")

	// Check for required build tools
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required to build whisper.cpp: %w", err)
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		return fmt.Errorf("cmake is required to build whisper.cpp: %w", err)
	}
	if _, err := exec.LookPath("g++"); err != nil {
		if _, err := exec.LookPath("clang++"); err != nil {
			return fmt.Errorf("g++ or clang++ is required to build whisper.cpp")
		}
	}

	// Clone whisper.cpp
	repoDir := filepath.Join(e.tempDir, "whisper.cpp")
	os.RemoveAll(repoDir) // Clean up any previous attempt

	fmt.Println("Cloning whisper.cpp repository...")
	cmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/ggml-org/whisper.cpp.git", repoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone whisper.cpp: %w", err)
	}

	// Build whisper.cpp using cmake
	fmt.Println("Building whisper.cpp (this may take a few minutes)...")

	// cmake -B build
	cmd = exec.Command("cmake", "-B", "build")
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure whisper.cpp: %w", err)
	}

	// cmake --build build --config Release
	cmd = exec.Command("cmake", "--build", "build", "--config", "Release", "-j")
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build whisper.cpp: %w", err)
	}

	// The binary is now in build/bin/whisper-cli
	srcBinary := filepath.Join(repoDir, "build", "bin", "whisper-cli")
	destBinary := filepath.Join(e.whisperDir, "whisper-cli")

	if err := copyFile(srcBinary, destBinary); err != nil {
		return fmt.Errorf("failed to copy whisper binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(destBinary, 0755); err != nil {
		return fmt.Errorf("failed to make whisper executable: %w", err)
	}

	// Copy shared libraries (needed for whisper-cli to run)
	libDir := filepath.Join(e.whisperDir, "lib")
	os.MkdirAll(libDir, 0755)

	// Copy all .so files from build directories
	buildLibDirs := []string{
		filepath.Join(repoDir, "build", "src"),
		filepath.Join(repoDir, "build", "ggml", "src"),
	}

	for _, dir := range buildLibDirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.so*"))
		for _, f := range files {
			destLib := filepath.Join(libDir, filepath.Base(f))
			copyFile(f, destLib)
		}
	}

	// Create a wrapper script that sets LD_LIBRARY_PATH
	wrapperScript := filepath.Join(e.whisperDir, "whisper")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH="%s:$LD_LIBRARY_PATH"
exec "%s" "$@"
`, libDir, destBinary)
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to create wrapper script: %w", err)
	}

	// Clean up source
	os.RemoveAll(repoDir)

	// Update engine path to use the wrapper
	e.whisperPath = wrapperScript
	fmt.Printf("Whisper.cpp built and installed to %s\n", wrapperScript)

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// DownloadPiperBinary downloads and extracts the piper binary
func (e *Engine) DownloadPiperBinary(progress DownloadProgressFunc) error {
	// Determine platform
	var url string
	var binaryName string
	var isZip bool

	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			url = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz"
			binaryName = "piper"
		} else if runtime.GOARCH == "arm64" {
			url = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_aarch64.tar.gz"
			binaryName = "piper"
		} else {
			return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			url = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_macos_aarch64.tar.gz"
		} else {
			url = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_macos_x64.tar.gz"
		}
		binaryName = "piper"
	case "windows":
		url = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_windows_amd64.zip"
		binaryName = "piper.exe"
		isZip = true
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	fmt.Printf("Downloading Piper from %s\n", url)

	// Download to temp file
	ext := ".tar.gz"
	if isZip {
		ext = ".zip"
	}
	tempPath := filepath.Join(e.tempDir, "piper"+ext)
	if err := downloadFile(url, tempPath, progress); err != nil {
		return fmt.Errorf("failed to download piper: %w", err)
	}
	defer os.Remove(tempPath)

	// Extract
	if isZip {
		if err := extractZip(tempPath, e.piperDir, binaryName); err != nil {
			return fmt.Errorf("failed to extract piper: %w", err)
		}
	} else {
		if err := extractTarGz(tempPath, e.piperDir); err != nil {
			return fmt.Errorf("failed to extract piper: %w", err)
		}
	}

	// Make executable
	piperPath := filepath.Join(e.piperDir, binaryName)
	// Check if piper is in a subdirectory (piper releases extract to piper/)
	if _, err := os.Stat(piperPath); os.IsNotExist(err) {
		piperPath = filepath.Join(e.piperDir, "piper", binaryName)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(piperPath, 0755); err != nil {
			return fmt.Errorf("failed to make piper executable: %w", err)
		}
	}

	// Create symlinks for shared libraries (piper needs versioned libs)
	if runtime.GOOS == "linux" {
		piperDir := filepath.Dir(piperPath)
		createLibSymlinks(piperDir)
	}

	// Update engine path
	e.piperPath = piperPath
	fmt.Printf("Piper installed to %s\n", piperPath)

	return nil
}

// createLibSymlinks creates symlinks for versioned shared libraries
func createLibSymlinks(dir string) {
	// Common piper libraries that need symlinks
	libs := map[string][]string{
		"libpiper_phonemize.so.1.2.0": {"libpiper_phonemize.so.1", "libpiper_phonemize.so"},
		"libonnxruntime.so.1.14.1":    {"libonnxruntime.so.1", "libonnxruntime.so"},
		"libespeak-ng.so.1.52.0.1":    {"libespeak-ng.so.1", "libespeak-ng.so"},
	}

	for src, links := range libs {
		srcPath := filepath.Join(dir, src)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		prevLink := src
		for _, link := range links {
			linkPath := filepath.Join(dir, link)
			os.Remove(linkPath) // Remove existing link
			os.Symlink(prevLink, linkPath)
			prevLink = link
		}
	}
}

// extractZip extracts a zip file, looking for the specified binary
func extractZip(zipPath, destDir, binaryName string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Extract all files
		fpath := filepath.Join(destDir, filepath.Base(f.Name))

		// If looking for binary, prioritize it
		if strings.HasSuffix(f.Name, binaryName) || filepath.Base(f.Name) == binaryName {
			fpath = filepath.Join(destDir, binaryName)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// extractTarGz extracts a tar.gz file
func extractTarGz(tarPath, destDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			// Preserve executable permissions
			if header.Mode&0111 != 0 {
				os.Chmod(target, 0755)
			}
		}
	}

	return nil
}

// extractZipAll extracts all files from a zip to the destination directory (flat)
func extractZipAll(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	os.MkdirAll(destDir, 0755)

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Extract to flat directory using just the filename
		fpath := filepath.Join(destDir, filepath.Base(f.Name))

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// extractTarGzAll extracts all files from a tar.gz to the destination directory (flat)
func extractTarGzAll(tarPath, destDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	os.MkdirAll(destDir, 0755)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Extract to flat directory using just the filename
		target := filepath.Join(destDir, filepath.Base(header.Name))

		outFile, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return err
		}
		outFile.Close()

		// Preserve executable permissions
		if header.Mode&0111 != 0 {
			os.Chmod(target, 0755)
		}
	}

	return nil
}

// downloadFile downloads a file with progress reporting
func downloadFile(url, destPath string, progress DownloadProgressFunc) error {
	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Start download
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength

	// Copy with progress
	if progress != nil {
		var downloaded int64
		buf := make([]byte, 32*1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				_, writeErr := out.Write(buf[:n])
				if writeErr != nil {
					return writeErr
				}
				downloaded += int64(n)
				progress(downloaded, total)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}
	} else {
		if _, err := io.Copy(out, resp.Body); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
	}

	return nil
}

// ListAvailableWhisperModels returns all downloadable whisper models
func ListAvailableWhisperModels() []WhisperModelInfo {
	var models []WhisperModelInfo
	for _, m := range WhisperModels {
		models = append(models, m)
	}
	return models
}

// ListAvailablePiperVoices returns all downloadable piper voices
func ListAvailablePiperVoices() []PiperVoiceInfo {
	var voices []PiperVoiceInfo
	for _, v := range PiperVoices {
		voices = append(voices, v)
	}
	return voices
}
