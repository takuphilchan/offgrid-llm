package audio

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// Metadata-only readiness: model loading and API validation remain worker work.
// Extend this version set only after reviewing the upstream frontend contract.
const speechModuleProbe = `
import importlib.util, importlib.metadata, json
found = {m:importlib.util.find_spec(m) is not None for m in ('qwen_asr','qwen_tts','sherpa_onnx','soundfile')}
try:
    version = importlib.metadata.version('sherpa-onnx')
except importlib.metadata.PackageNotFoundError:
    version = ''
found['kokoro_legacy'] = version == '1.12.20'
found['kokoro_lexicon'] = version == '2.0.0'
print(json.dumps(found))
`

func kokoroRuntimeAvailable(modules map[string]bool, manifest models.ModelPackageManifest) bool {
	if !modules["sherpa_onnx"] || !modules["soundfile"] {
		return false
	}
	if modules["kokoro_legacy"] {
		return true
	}
	if modules["kokoro_lexicon"] {
		for _, a := range manifest.Artifacts {
			if a.Role == "phonemizer_data" && strings.HasPrefix(filepath.Base(a.Path), "lexicon-") && strings.HasSuffix(a.Path, ".txt") {
				return true
			}
		}
	}
	return false
}

// KokoroConfig comes only from a validated, leased package, never HTTP paths.
type KokoroConfig struct {
	Model, Voices, Tokens, DataDir, DictDir, Lexicon string
	VoiceIDs                                         []string
}

func kokoroConfig(profile SpeechProfile) (*KokoroConfig, error) {
	cfg := &KokoroConfig{}
	var lexicons []string
	for _, a := range profile.manifest.Artifacts {
		file := filepath.Join(profile.dir, filepath.FromSlash(a.Path))
		switch a.Role {
		case "weights":
			cfg.Model = file
		case "voices":
			cfg.Voices = file
		case "tokens":
			cfg.Tokens = file
		case "phonemizer_data":
			for _, directory := range []string{"espeak-ng-data", "dict"} {
				parts := strings.Split(a.Path, "/")
				for i, part := range parts[:len(parts)-1] {
					if part == directory {
						root := filepath.Join(profile.dir, filepath.FromSlash(strings.Join(parts[:i+1], "/")))
						if directory == "dict" {
							cfg.DictDir = root
						} else {
							cfg.DataDir = root
						}
					}
				}
			}
			if strings.HasPrefix(filepath.Base(file), "lexicon-") && strings.HasSuffix(file, ".txt") {
				// sherpa's lexicon list uses commas as delimiters.
				if strings.Contains(file, ",") {
					return nil, fmt.Errorf("Kokoro lexicon paths cannot contain commas")
				}
				lexicons = append(lexicons, file)
			}
		}
	}
	for _, voice := range profile.Voices {
		id, err := strconv.Atoi(voice.ID)
		if err != nil || id < 0 || strconv.Itoa(id) != voice.ID {
			return nil, fmt.Errorf("Kokoro requires declared numeric voice IDs")
		}
		cfg.VoiceIDs = append(cfg.VoiceIDs, voice.ID)
	}
	cfg.Lexicon = strings.Join(lexicons, ",")
	if cfg.Model == "" || cfg.Voices == "" || cfg.Tokens == "" || (cfg.DataDir == "" && cfg.Lexicon == "") || len(cfg.VoiceIDs) == 0 {
		return nil, fmt.Errorf("Kokoro package needs weights, voices, tokens, phonemizer data and declared voice IDs")
	}
	return cfg, nil
}

const kokoroWorkerScript = `
def synthesize_kokoro(req):
    global model
    import sherpa_onnx
    import soundfile as sf
    cfg = req["Kokoro"]
    version = getattr(sherpa_onnx, '__version__', '')
    if version not in ('1.12.20', '2.0.0'):
        raise RequestError("Kokoro requires a reviewed sherpa-onnx runtime version (1.12.20 or 2.0.0).")
    if version == '2.0.0' and not cfg['Lexicon']:
        raise RequestError("This sherpa-onnx runtime requires Kokoro package lexicons; espeak data alone is unsupported.")
    if version == '2.0.0' and not hasattr(sherpa_onnx, 'GenerationConfig'):
        raise RequestError("Kokoro runtime is missing its generation API.")
    voice = req.get("Voice") or cfg["VoiceIDs"][0]
    if voice not in cfg["VoiceIDs"]:
        raise RequestError("This voice is not available in the selected model.")
    if model is None:
        kokoro = sherpa_onnx.OfflineTtsKokoroModelConfig(
            model=cfg["Model"], voices=cfg["Voices"], tokens=cfg["Tokens"],
            data_dir=cfg["DataDir"], dict_dir=cfg["DictDir"], lexicon=cfg["Lexicon"])
        config = sherpa_onnx.OfflineTtsConfig(
            model=sherpa_onnx.OfflineTtsModelConfig(kokoro=kokoro, num_threads=4, provider="cpu", debug=False),
            max_num_sentences=1)
        if not config.validate():
            raise RequestError("Kokoro package is incompatible with this speech runtime.")
        model = sherpa_onnx.OfflineTts(config)
    sid = int(voice)
    if sid >= model.num_speakers:
        raise RequestError("This voice is not available in the selected model.")
    speed = req.get("Speed") or 1.0
    # Newer sherpa releases expose GenerationConfig; retain the reviewed older
    # API as well. Both paths use local artifacts and the same owned process.
    if hasattr(sherpa_onnx, "GenerationConfig"):
        generation = sherpa_onnx.GenerationConfig()
        generation.sid, generation.speed = sid, speed
        audio = model.generate(req["Text"], generation)
    else:
        audio = model.generate(req["Text"], sid=sid, speed=speed)
    if len(audio.samples) == 0 or audio.sample_rate <= 0:
        raise RequestError("Speech runtime produced no audio.")
    sf.write(req["InputPath"], audio.samples, audio.sample_rate, subtype="PCM_16", format="WAV")
    return dict(ok=True, output_path=req["InputPath"])

`
