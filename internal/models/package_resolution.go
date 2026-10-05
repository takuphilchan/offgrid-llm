package models

import (
	"context"
	"crypto/sha1" // Git blob identity for small data, not package integrity.
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxHubMetadata = 16 << 20
const maxHubFiles = 20000
const maxResolutionData = 64 << 20
const maxSmallArtifact = 8 << 20

type ModelAcquisitionError struct {
	Code    string
	Message string
}

func (e *ModelAcquisitionError) Error() string    { return e.Message }
func acquisitionError(code, message string) error { return &ModelAcquisitionError{code, message} }

type hubPackageFile struct {
	Type string `json:"type"`
	Path string `json:"path"`
	OID  string `json:"oid"`
	Size int64  `json:"size"`
	LFS  *struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs,omitempty"`
}
type hubPackageMetadata struct {
	ID       string  `json:"id"`
	SHA      string  `json:"sha"`
	Private  bool    `json:"private"`
	Gated    HFGated `json:"gated"`
	Disabled bool    `json:"disabled"`
	Card     struct {
		License  string          `json:"license"`
		Language json.RawMessage `json:"language"`
	} `json:"cardData"`
}
type PackageChoice struct {
	ID           string `json:"id"`
	Architecture string `json:"architecture"`
	Name         string `json:"name"`
	Supported    bool   `json:"supported"`
	Reason       string `json:"reason,omitempty"`
}
type PackageDiscovery struct {
	Repository string          `json:"repository"`
	Revision   string          `json:"revision"`
	License    string          `json:"license"`
	Choices    []PackageChoice `json:"choices"`
}
type PackageResolutionRequest struct {
	Repository   string `json:"repository"`
	Revision     string `json:"revision"`
	Variant      string `json:"variant"`
	Architecture string `json:"architecture"`
}
type PackageResolution struct {
	Manifest      ModelPackageManifest `json:"manifest"`
	Provenance    ModelProvenance      `json:"provenance"`
	SourceNotices []string             `json:"source_notices"`
	TransferBytes int64                `json:"transfer_bytes"`
}

// Hosts are reviewed distribution infrastructure, not arbitrary model-supplied
// URLs. Preserve DefaultTransport proxy settings; do not bypass the user's VPN.
func validHubURL(u *url.URL, metadata bool) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if h == "huggingface.co" {
		return true
	}
	if metadata {
		return false
	}
	return h == "cdn-lfs.huggingface.co" || h == "cdn-lfs.hf.co" || h == "cdn-lfs-us-1.hf.co" || h == "cdn-lfs-eu-1.hf.co" || h == "cas-bridge.xethub.hf.co" || h == "us.aws.cdn.hf.co" || h == "eu.aws.cdn.hf.co"
}

func (hf *HuggingFaceClient) packageClient(metadata bool) *http.Client {
	c := *hf.client
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !validHubURL(req.URL, metadata) {
			return acquisitionError("unsafe_source", "Model source redirected outside supported HTTPS distribution hosts.")
		}
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		req.Header.Del("Proxy-Authorization")
		return nil
	}
	return &c
}

func (hf *HuggingFaceClient) packageGet(ctx context.Context, address string, limit int64, metadata bool) ([]byte, http.Header, error) {
	u, err := url.Parse(address)
	if err != nil || !validHubURL(u, metadata) {
		return nil, nil, acquisitionError("unsafe_source", "Only supported public Hugging Face HTTPS sources are allowed.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "OffGrid-model-resolver/1")
	resp, err := hf.packageClient(metadata).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		var safe *ModelAcquisitionError
		if errors.As(err, &safe) {
			return nil, nil, safe
		}
		return nil, nil, acquisitionError("source_unavailable", "Could not reach the model source. Check connectivity or proxy settings.")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, nil, acquisitionError("public_access_required", "Private or gated repositories are not supported. Choose a public model or import an authorized local package.")
	}
	if resp.StatusCode != 200 {
		return nil, nil, acquisitionError("source_unavailable", fmt.Sprintf("Model source returned HTTP %d.", resp.StatusCode))
	}
	if resp.ContentLength > limit {
		return nil, nil, acquisitionError("metadata_limit", "Repository data exceeds the resolution limit.")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, acquisitionError("source_unavailable", "Model source response was interrupted.")
	}
	if int64(len(b)) > limit {
		return nil, nil, acquisitionError("metadata_limit", "Repository data exceeds the resolution limit.")
	}
	return b, resp.Header, nil
}

func (hf *HuggingFaceClient) packageSnapshot(ctx context.Context, repo, revision string) (hubPackageMetadata, []hubPackageFile, error) {
	var meta hubPackageMetadata
	if !ValidHubRepository(repo) || (revision != "" && !sourceRevision.MatchString(revision)) {
		return meta, nil, acquisitionError("invalid_source", "Select a repository and an immutable revision returned by discovery.")
	}
	address := "https://huggingface.co/api/models/" + repo
	if revision != "" {
		address += "/revision/" + revision
	}
	b, _, err := hf.packageGet(ctx, address, maxHubMetadata, true)
	if err != nil {
		return meta, nil, err
	}
	if json.Unmarshal(b, &meta) != nil || meta.ID != repo || !sourceRevision.MatchString(meta.SHA) || (revision != "" && meta.SHA != revision) {
		return meta, nil, acquisitionError("source_identity_mismatch", "Repository or revision identity changed. Discover the source again.")
	}
	if meta.Private || bool(meta.Gated) || meta.Disabled {
		return meta, nil, acquisitionError("public_access_required", "Only public, ungated model repositories are supported.")
	}
	base := "https://huggingface.co/api/models/" + repo + "/tree/" + meta.SHA
	next := base + "?recursive=true&limit=1000"
	files := []hubPackageFile{}
	seenURLs, seenPaths := map[string]bool{}, map[string]bool{}
	remaining := int64(maxHubMetadata - len(b))
	for page := 0; next != ""; page++ {
		if page >= 32 || seenURLs[next] || remaining <= 0 {
			return meta, nil, acquisitionError("incomplete_listing", "Repository listing is incomplete or exceeds supported limits.")
		}
		seenURLs[next] = true
		b, headers, err := hf.packageGet(ctx, next, remaining, true)
		if err != nil {
			return meta, nil, err
		}
		remaining -= int64(len(b))
		var items []hubPackageFile
		if json.Unmarshal(b, &items) != nil || len(items) > 1000 {
			return meta, nil, acquisitionError("incomplete_listing", "Invalid repository file listing.")
		}
		for _, f := range items {
			if !validArtifactPath(f.Path) || seenPaths[strings.ToLower(f.Path)] || (f.Type != "file" && f.Type != "directory") {
				return meta, nil, acquisitionError("unsafe_layout", "Repository contains duplicate, unsafe, or unsupported entries.")
			}
			seenPaths[strings.ToLower(f.Path)] = true
			if len(seenPaths) > maxHubFiles {
				return meta, nil, acquisitionError("incomplete_listing", "Repository has too many files to resolve safely.")
			}
			if f.Type == "file" {
				files = append(files, f)
			}
		}
		next = ""
		if link := headers.Get("Link"); link != "" {
			// Never follow an arbitrary pagination host or a different revision.
			for _, part := range strings.Split(link, ",") {
				if !strings.Contains(part, `rel="next"`) {
					continue
				}
				start, end := strings.Index(part, "<"), strings.Index(part, ">")
				if start < 0 || end <= start || next != "" {
					return meta, nil, acquisitionError("incomplete_listing", "Invalid pagination metadata.")
				}
				u, err := url.Parse(part[start+1 : end])
				if err != nil || !validHubURL(u, true) || u.Scheme+"://"+u.Host+u.Path != base || u.Query().Get("recursive") != "true" {
					return meta, nil, acquisitionError("unsafe_source", "Pagination left the selected repository revision.")
				}
				next = u.String()
			}
			if next == "" {
				return meta, nil, acquisitionError("incomplete_listing", "Unrecognized pagination metadata; refusing a partial package.")
			}
		}
	}
	return meta, files, nil
}

var whisperEnglishFile = regexp.MustCompile(`^ggml-(tiny|base|small|medium)\.en(-q[0-9]_[0-9])?\.bin$`)
var zipformerEncoder = regexp.MustCompile(`^encoder-(epoch-[0-9]+-avg-[0-9]+-chunk-[0-9]+-left-[0-9]+)(\.int8)?\.onnx$`)

func packageChoices(repo string, files []hubPackageFile) []PackageChoice {
	index := map[string]hubPackageFile{}
	for _, f := range files {
		index[f.Path] = f
	}
	result := []PackageChoice{}
	// Only official Qwen3 repositories are recognized by this recipe. This is
	// intentionally repository-scoped: a filename pattern must never authorize
	// arbitrary downloaded code as a speech runtime.
	qwenRepo := strings.ToLower(repo)
	if strings.HasPrefix(qwenRepo, "qwen/qwen3-asr-") && index["config.json"].Path != "" && index["preprocessor_config.json"].Path != "" {
		for p, f := range index {
			if f.Type == "file" && strings.HasSuffix(strings.ToLower(p), ".safetensors") {
				result = append(result, PackageChoice{ID: "qwen3-asr", Architecture: "qwen3-asr", Name: "Qwen3-ASR package", Supported: true})
				break
			}
		}
	}
	if strings.HasPrefix(qwenRepo, "qwen/qwen3-tts-") && index["config.json"].Path != "" {
		for p, f := range index {
			if f.Type == "file" && strings.HasSuffix(strings.ToLower(p), ".safetensors") {
				result = append(result, PackageChoice{ID: "qwen3-tts", Architecture: "qwen3-tts", Name: "Qwen3-TTS package", Supported: true})
				break
			}
		}
	}
	for _, f := range files {
		dir, base := path.Dir(f.Path), path.Base(f.Path)
		choice := PackageChoice{ID: f.Path, Name: f.Path, Supported: true}
		switch {
		case whisperEnglishFile.MatchString(base):
			choice.Architecture = "whisper"
		case strings.HasSuffix(base, ".onnx") && index[f.Path+".json"].Path != "":
			choice.Architecture = "piper"
			if index[path.Join(dir, "MODEL_CARD")].Path == "" {
				choice.Supported = false
				choice.Reason = "voice_license_missing"
			}
		case zipformerEncoder.MatchString(base):
			choice.Architecture = "zipformer-streaming"
			suffix := strings.TrimPrefix(base, "encoder-")
			for _, needed := range []string{"decoder-" + suffix, "joiner-" + suffix, "tokens.txt"} {
				if index[path.Join(dir, needed)].Path == "" {
					choice.Supported = false
					choice.Reason = "incomplete_dependencies"
				}
			}
		case (base == "model.onnx" || base == "model.int8.onnx") && index[path.Join(dir, "voices.bin")].Path != "":
			choice.Architecture = "kokoro"
			for _, needed := range []string{"tokens.txt", "espeak-ng-data/phondata", "espeak-ng-data/phonindex", "espeak-ng-data/phontab", "espeak-ng-data/en_dict", "espeak-ng-data/lang/gmw/en", "LICENSE"} {
				if index[path.Join(dir, needed)].Path == "" {
					choice.Supported = false
					choice.Reason = "incomplete_dependencies"
				}
			}
		default:
			continue
		}
		result = append(result, choice)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (hf *HuggingFaceClient) DiscoverPackages(ctx context.Context, repo string) (PackageDiscovery, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	meta, files, err := hf.packageSnapshot(ctx, repo, "")
	if err != nil {
		return PackageDiscovery{}, err
	}
	choices := packageChoices(repo, files)
	if len(choices) == 0 && len(files) > 0 {
		// Keep discovery informative without turning arbitrary Transformers,
		// NeMo, pickle, or repository-code layouts into installable packages.
		choices = []PackageChoice{{ID: "repository", Architecture: "unsupported", Name: "Repository files found", Supported: false, Reason: "no_reviewed_adapter"}}
	}
	return PackageDiscovery{Repository: repo, Revision: meta.SHA, License: meta.Card.License, Choices: choices}, nil
}

// Revisions identify reviewed adapter requirements, not installed executables.
func recipeRuntime(architecture string) PackageRuntime {
	switch architecture {
	case "whisper":
		return PackageRuntime{Adapter: "whisper.cpp", Revision: "4979e04f5dcaccb36057e059bbaed8a2f5288315"}
	case "piper":
		return PackageRuntime{Adapter: "piper", Revision: "fee9b9cefae4ebf9e196cfe994dea418f051506c"}
	case "qwen3-asr":
		return PackageRuntime{Adapter: "qwen3-asr", Revision: "7c6daf77a2421100f5fb066495372c00129d39ff"}
	case "qwen3-tts":
		return PackageRuntime{Adapter: "qwen3-tts", Revision: "022e286b98fbec7e1e916cb940cdf532cd9f488e"}
	default:
		return PackageRuntime{Adapter: "sherpa-onnx", Revision: "26aa2fa93210376a89de3a65a1a4dd320c37f5e9"}
	}
}

type packageResolutionData struct {
	mu             sync.Mutex
	hf             *HuggingFaceClient
	ctx            context.Context
	repo, revision string
	remaining      int64
	cache          map[string][]byte
}

func (r *packageResolutionData) data(f hubPackageFile) ([]byte, error) {
	r.mu.Lock()
	if b, ok := r.cache[f.Path]; ok {
		r.mu.Unlock()
		return b, nil
	}
	if f.Size <= 0 || f.Size > maxSmallArtifact || f.Size > r.remaining {
		r.mu.Unlock()
		return nil, acquisitionError("missing_artifact_identity", "Artifact has no usable digest, or exceeds the bounded data-resolution limit.")
	}
	r.remaining -= f.Size
	r.mu.Unlock()
	u := "https://huggingface.co/" + r.repo + "/resolve/" + r.revision + "/" + escapeHubPath(f.Path)
	b, _, err := r.hf.packageGet(r.ctx, u, f.Size, false)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) != f.Size {
		return nil, acquisitionError("source_identity_mismatch", "Source file size changed during resolution.")
	}
	if f.LFS != nil {
		h := sha256.Sum256(b)
		if f.LFS.Size != f.Size || hex.EncodeToString(h[:]) != f.LFS.OID {
			return nil, acquisitionError("source_identity_mismatch", "Source digest does not match its immutable repository metadata.")
		}
	} else {
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", len(b))
		h.Write(b)
		if len(f.OID) != 40 || hex.EncodeToString(h.Sum(nil)) != f.OID {
			return nil, acquisitionError("source_identity_mismatch", "Source Git object identity does not match its immutable metadata.")
		}
	}
	r.mu.Lock()
	r.cache[f.Path] = b
	r.mu.Unlock()
	return b, nil
}
func escapeHubPath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func (hf *HuggingFaceClient) ResolvePackage(ctx context.Context, req PackageResolutionRequest) (PackageResolution, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	var result PackageResolution
	if !sourceRevision.MatchString(req.Revision) || !validArtifactPath(req.Variant) {
		return result, acquisitionError("invalid_source", "A discovered variant and immutable source revision are required.")
	}
	meta, files, err := hf.packageSnapshot(ctx, req.Repository, req.Revision)
	if err != nil {
		return result, err
	}
	var choice *PackageChoice
	for _, c := range packageChoices(req.Repository, files) {
		if c.ID == req.Variant && c.Architecture == req.Architecture {
			choice = &c
			break
		}
	}
	if choice == nil || !choice.Supported {
		return result, acquisitionError("unsupported_layout", "This variant does not have a complete supported speech layout. Choose another variant.")
	}
	index := map[string]hubPackageFile{}
	for _, f := range files {
		index[f.Path] = f
	}
	r := packageResolutionData{hf: hf, ctx: ctx, repo: req.Repository, revision: meta.SHA, remaining: maxResolutionData, cache: map[string][]byte{}}
	idHash := sha256.Sum256([]byte(req.Repository + "/" + req.Architecture + "/" + req.Variant))
	stem := strings.ToLower(localStem.ReplaceAllString(strings.TrimSuffix(path.Base(req.Variant), path.Ext(req.Variant)), "-"))
	if len(stem) > 48 {
		stem = stem[:48]
	}
	m := ModelPackageManifest{SchemaVersion: PackageManifestVersion, ID: fmt.Sprintf("%s-%x", stem, idHash[:6]), Revision: meta.SHA, Name: req.Repository + " · " + path.Base(req.Variant), Architecture: req.Architecture, Runtime: recipeRuntime(req.Architecture), Languages: []string{"en"}, SampleRates: []int{16000}, License: meta.Card.License, Artifacts: []PackageArtifact{}}
	dir := path.Dir(req.Variant)
	roles := map[string]string{}
	if req.Architecture != "qwen3-asr" && req.Architecture != "qwen3-tts" {
		roles[req.Variant] = "weights"
	}
	switch req.Architecture {
	case "whisper":
		m.Capabilities = []ModelCapability{CapabilityTranscription}
	case "piper":
		m.Capabilities = []ModelCapability{CapabilitySynthesis}
		roles[req.Variant+".json"] = "config"
		roles[path.Join(dir, "MODEL_CARD")] = "license"
		config, err := r.data(index[req.Variant+".json"])
		if err != nil {
			return result, err
		}
		var c struct {
			Audio struct {
				SampleRate int `json:"sample_rate"`
			} `json:"audio"`
			Language struct {
				Code string `json:"code"`
			} `json:"language"`
			NumSpeakers int            `json:"num_speakers"`
			Speakers    map[string]int `json:"speaker_id_map"`
			PhonemeType string         `json:"phoneme_type"`
		}
		if json.Unmarshal(config, &c) != nil || c.NumSpeakers < 1 || c.NumSpeakers > 1024 || c.Language.Code == "" || c.PhonemeType != "espeak" {
			return result, acquisitionError("unsupported_layout", "Piper configuration is missing required language, speaker, or supported phonemizer metadata.")
		}
		language := strings.ReplaceAll(c.Language.Code, "_", "-")
		m.Languages = []string{language}
		m.SampleRates = []int{c.Audio.SampleRate}
		for i := 0; i < c.NumSpeakers; i++ {
			m.Voices = append(m.Voices, PackageVoice{ID: fmt.Sprint(i), Language: language})
		}
		m.License = "LicenseRef-Voice-Model-Card"
		card, err := r.data(index[path.Join(dir, "MODEL_CARD")])
		if err != nil {
			return result, err
		}
		if len(card) > 32768 {
			return result, acquisitionError("metadata_limit", "Voice license notice exceeds the supported preview limit.")
		}
		result.SourceNotices = append(result.SourceNotices, string(card), "Piper runtime requires separately packaged eSpeak phonemizer data. Voice and runtime licenses differ; review the voice model card.")
	case "zipformer-streaming":
		m.Capabilities = []ModelCapability{CapabilityTranscription, CapabilityStreamingRecognition}
		roles[req.Variant] = "encoder"
		suffix := strings.TrimPrefix(path.Base(req.Variant), "encoder-")
		roles[path.Join(dir, "decoder-"+suffix)] = "decoder"
		roles[path.Join(dir, "joiner-"+suffix)] = "joiner"
		roles[path.Join(dir, "tokens.txt")] = "tokens"
		// Unknown languages cannot be inferred from the ONNX suffix. The initial
		// reviewed English profile is explicit; other repos need model-card data.
		if req.Repository != "csukuangfj/sherpa-onnx-streaming-zipformer-en-2023-06-26" {
			var langs []string
			if json.Unmarshal(meta.Card.Language, &langs) != nil {
				var lang string
				if json.Unmarshal(meta.Card.Language, &lang) == nil {
					langs = []string{lang}
				}
			}
			if len(langs) == 0 {
				return result, acquisitionError("unsupported_layout", "Streaming model needs explicit language metadata.")
			}
			m.Languages = langs
		}
	case "kokoro":
		m.Capabilities = []ModelCapability{CapabilitySynthesis}
		m.SampleRates = []int{24000}
		m.Voices = []PackageVoice{{ID: "0", Language: "en"}}
		roles[path.Join(dir, "voices.bin")] = "voices"
		roles[path.Join(dir, "tokens.txt")] = "tokens"
		roles[path.Join(dir, "LICENSE")] = "license"
		license, err := r.data(index[path.Join(dir, "LICENSE")])
		if err != nil {
			return result, err
		}
		if !strings.Contains(string(license), "Apache License") || !strings.Contains(string(license), "Version 2.0") {
			return result, acquisitionError("unsupported_license", "Kokoro layout needs a reviewed model license.")
		}
		m.License = "Apache-2.0 AND GPL-3.0-or-later"
		for _, f := range files {
			if strings.HasPrefix(f.Path, path.Join(dir, "espeak-ng-data")+"/") {
				roles[f.Path] = "phonemizer_data"
			}
		}
		for _, name := range []string{"lexicon-us-en.txt", "lexicon-gb-en.txt"} {
			if index[path.Join(dir, name)].Path != "" {
				roles[path.Join(dir, name)] = "phonemizer_data"
			}
		}
		result.SourceNotices = append(result.SourceNotices, "Initial Kokoro profile exposes English voice 0 only. eSpeak data has separate GPL terms; model license is Apache-2.0. No language or hardware qualification is implied.")
	case "qwen3-asr", "qwen3-tts":
		// Admit declarative model assets only. Repository Python, pickle, and
		// executable files are deliberately excluded; the pinned adapter is
		// supervised by OffGrid and is not downloaded from the model repository.
		if req.Architecture == "qwen3-asr" {
			m.Capabilities = []ModelCapability{CapabilityTranscription}
			if index["preprocessor_config.json"].Path == "" {
				return result, acquisitionError("unsupported_layout", "Qwen3-ASR requires preprocessor_config.json.")
			}
		} else {
			m.Capabilities = []ModelCapability{CapabilitySynthesis, CapabilityIncrementalSynthesis}
			m.Voices = []PackageVoice{{ID: "default", Language: "en"}}
		}
		roles["config.json"] = "config"
		for _, f := range files {
			lower := strings.ToLower(f.Path)
			ext := strings.ToLower(path.Ext(f.Path))
			switch {
			case strings.HasSuffix(lower, ".safetensors"):
				roles[f.Path] = "weights"
			case lower == "readme.md":
				roles[f.Path] = "license"
			case ext == ".json" || ext == ".txt" || ext == ".model" || ext == ".merges" || ext == ".vocab":
				if strings.Contains(lower, "token") || strings.Contains(lower, "vocab") || strings.HasSuffix(lower, "merges.txt") {
					roles[f.Path] = "tokenizer"
				} else if strings.Contains(lower, "codec") || strings.Contains(lower, "speech_tokenizer") {
					roles[f.Path] = "codec"
				} else {
					roles[f.Path] = "config"
				}
			}
		}
		m.Languages = []string{"en"}
		if req.Architecture == "qwen3-asr" {
			m.SampleRates = []int{16000}
		} else {
			m.SampleRates = []int{24000}
		}
		m.License = meta.Card.License
		result.SourceNotices = append(result.SourceNotices, "Qwen repository code is excluded. This package requires the pinned OffGrid Qwen runtime adapter; installation is not runtime qualification.")
	}
	if strings.TrimSpace(m.License) == "" {
		return result, acquisitionError("license_missing", "Repository must declare its model license before installation.")
	}
	// Include the upstream source notice if present; no scripts, pickles, or repo
	// dependency installers are included or executed.
	if f := index["README.md"]; f.Path != "" && f.Size > 0 && f.Size <= 32768 {
		roles[f.Path] = "license"
	}
	paths := make([]string, 0, len(roles))
	for p := range roles {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if len(paths) > MaxPackageArtifacts {
		return result, acquisitionError("metadata_limit", "Package has too many required artifacts.")
	}
	resolveArtifact := func(p string) (PackageArtifact, error) {
		f := index[p]
		a := PackageArtifact{Path: p, Role: roles[p], Size: f.Size, License: m.License, Source: &ArtifactSource{Repository: "https://huggingface.co/" + req.Repository, Revision: meta.SHA, Path: p}}
		if req.Architecture == "kokoro" {
			a.License = "Apache-2.0"
			if strings.Contains(p, "espeak-ng-data/") {
				a.License = "GPL-3.0-or-later"
			}
		}
		if f.LFS != nil {
			if f.Size <= 0 || f.LFS.Size != f.Size || !packageDigest.MatchString(f.LFS.OID) {
				return a, acquisitionError("missing_artifact_identity", "Required artifact has invalid size or SHA-256 metadata.")
			}
			a.SHA256 = f.LFS.OID
		} else {
			if a.Role == "weights" || a.Role == "encoder" || a.Role == "decoder" || a.Role == "joiner" || a.Role == "voices" {
				return a, acquisitionError("missing_artifact_identity", "Model weights require an upstream SHA-256 identity. They are not downloaded during discovery.")
			}
			b, err := r.data(f)
			if err != nil {
				return a, err
			}
			h := sha256.Sum256(b)
			a.SHA256 = hex.EncodeToString(h[:])
		}
		return a, nil
	}
	// Four bounded metadata workers keep large phonemizer packages usable without
	// multiplying the byte budget, deadline, or number of open connections.
	m.Artifacts = make([]PackageArtifact, len(paths))
	jobs := make(chan int, len(paths))
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	var group sync.WaitGroup
	var firstError error
	var once sync.Once
	for worker := 0; worker < 4; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				a, err := resolveArtifact(paths[i])
				if err != nil {
					once.Do(func() { firstError = err; cancel() })
					return
				}
				m.Artifacts[i] = a
			}
		}()
	}
	group.Wait()
	if firstError != nil {
		return result, firstError
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	for _, a := range m.Artifacts {
		result.TransferBytes += a.Size
	}
	if err := m.Validate(); err != nil {
		return result, acquisitionError("unsupported_layout", "Resolved artifacts do not form a valid supported package: "+err.Error())
	}
	result.Manifest = m
	result.Provenance = ModelProvenance{Kind: "repository_metadata", Repository: req.Repository, Revision: meta.SHA}
	return result, nil
}
