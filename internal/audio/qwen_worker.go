package audio

// The private stdout descriptor carries JSON only. Python and native-library
// banners go to stderr, never into protocol frames. Model loading is offline.
var qwenWorkerScript = []byte(`
import json, os, sys
protocol = os.fdopen(os.dup(sys.stdout.fileno()), "w", encoding="utf-8", buffering=1)
os.dup2(sys.stderr.fileno(), sys.stdout.fileno())
os.environ["HF_HUB_OFFLINE"] = "1"
os.environ["TRANSFORMERS_OFFLINE"] = "1"
os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"
os.environ.setdefault("OMP_NUM_THREADS", "4")
model = None

class RequestError(Exception):
    pass

# The HTTP API uses language codes; Qwen3-ASR accepts full language names.
# Keep this reviewed list explicit: unsupported African languages must fail,
# never silently become English or automatic detection.
ASR_LANGUAGES = dict(zip(
    "zh en yue ar de fr es pt id it ko ru th vi ja tr hi ms nl sv da fi pl cs fil fa el ro hu mk".split(),
    "Chinese English Cantonese Arabic German French Spanish Portuguese Indonesian Italian Korean Russian Thai Vietnamese Japanese Turkish Hindi Malay Dutch Swedish Danish Finnish Polish Czech Filipino Persian Greek Romanian Hungarian Macedonian".split()))

def asr_language(value):
    key = str(value or "").strip().lower()
    if key in ("", "auto"):
        return None
    if key in ASR_LANGUAGES:
        return ASR_LANGUAGES[key]
    for name in ASR_LANGUAGES.values():
        if key == name.lower():
            return name
    raise RequestError("This language is not supported by Qwen ASR. Select a compatible recognition model or omit the language for automatic detection.")

def asr_language_code(value):
    key = str(value or "").strip().lower()
    for code, name in ASR_LANGUAGES.items():
        if key in (code, name.lower()):
            return code
    return ""

def reply(**kw):
    protocol.write(json.dumps(kw, ensure_ascii=False) + "\n")
    protocol.flush()

def perform(req):
    global model
    op = req.get("Op")
    model_dir = req.get("ModelDir")
    if not model_dir or not os.path.isdir(model_dir):
        raise RequestError("Model package is unavailable. Check Models for repair options.")
    if op == "transcribe":
        language = asr_language(req.get("Language"))
        from qwen_asr import Qwen3ASRModel
        import torch
        torch.set_num_threads(4)
        if model is None:
            model = Qwen3ASRModel.from_pretrained(model_dir, dtype=torch.float32, device_map="cpu", local_files_only=True)
        results = model.transcribe(audio=req["InputPath"], language=language)
        result = results[0] if isinstance(results, (list, tuple)) else results
        text = result.get("text", "") if isinstance(result, dict) else getattr(result, "text", "")
        detected = result.get("language", "") if isinstance(result, dict) else getattr(result, "language", "")
        return dict(ok=True, text=text, language=asr_language_code(detected or language))
    if op == "synthesize":
        if req.get("Kind") == "kokoro":
            return synthesize_kokoro(req)
        with open(os.path.join(model_dir, "config.json"), encoding="utf-8") as f:
            config = json.load(f)
        if str(config.get("tts_model_type", "")).lower() != "custom_voice":
            raise RequestError("Read-aloud requires Qwen CustomVoice. Base requires reference audio and is not supported.")
        from qwen_tts import Qwen3TTSModel
        import torch
        import soundfile as sf
        torch.set_num_threads(4)
        if model is None:
            model = Qwen3TTSModel.from_pretrained(model_dir, device_map="cpu", dtype=torch.float32, local_files_only=True)
        speaker = req.get("Voice") or "Ryan"
        if speaker == "default": speaker = "Ryan"
        supported = model.get_supported_speakers()
        if supported and speaker.lower() not in [s.lower() for s in supported]:
            raise RequestError("This voice is not available in the selected model.")
        audio, sr = model.generate_custom_voice(text=req["Text"], language="Auto", speaker=speaker, max_new_tokens=2048)
        waveform = audio[0] if isinstance(audio, (list, tuple)) else audio
        # The service creates, owns and removes this unique output file.
        sf.write(req["InputPath"], waveform, sr, subtype="PCM_16", format="WAV")
        return dict(ok=True, output_path=req["InputPath"])
    raise RequestError("Unsupported speech operation.")

` + kokoroWorkerScript + `
for line in sys.stdin:
    try:
        reply(**perform(json.loads(line)))
    except RequestError as e:
        reply(ok=False, error=str(e))
    except ImportError:
        reply(ok=False, error="Speech runtime dependencies are missing. Repair the matching speech runtime.")
    except MemoryError:
        reply(ok=False, error="Not enough memory for speech. Close other model workloads and retry.")
    except Exception as e:
        # No transcript, prompt, local path, or model exception payload in logs.
        reply(ok=False, error="Speech inference failed (" + type(e).__name__ + "). Verify the model package and runtime compatibility.")
`)
