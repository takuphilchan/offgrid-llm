package models

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type packageFixtureTransport func(*http.Request) (*http.Response, error)

func (f packageFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type packageHubFixture struct {
	mu      sync.Mutex
	t       *testing.T
	meta    hubPackageMetadata
	files   []hubPackageFile
	data    map[string][]byte
	link    string
	fetches []string
}

const fixtureRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func newPackageHubFixture(t *testing.T, arch string) (*packageHubFixture, *HuggingFaceClient, string) {
	f := &packageHubFixture{t: t, data: map[string][]byte{}}
	f.meta.ID = "fixture/speech"
	f.meta.SHA = fixtureRevision
	f.meta.Card.License = "CC0-1.0"
	f.meta.Card.Language = json.RawMessage(`["en"]`)
	add := func(p, data string, weight bool) { f.add(p, []byte(data), weight) }
	variant := ""
	switch arch {
	case "whisper":
		variant = "ggml-tiny.en.bin"
		add(variant, "synthetic-not-real-weights", true)
	case "piper":
		variant = "en/voice.onnx"
		add(variant, "synthetic-not-real-weights", true)
		add(variant+".json", `{"audio":{"sample_rate":22050},"language":{"code":"en_US"},"num_speakers":1,"phoneme_type":"espeak"}`, false)
		add("en/MODEL_CARD", "Synthetic voice, CC0-1.0 test fixture", false)
	case "zipformer-streaming":
		variant = "encoder-epoch-99-avg-1-chunk-16-left-128.int8.onnx"
		for _, role := range []string{"encoder", "decoder", "joiner"} {
			add(role+"-epoch-99-avg-1-chunk-16-left-128.int8.onnx", role, true)
		}
		add("tokens.txt", "token 0", false)
	case "kokoro":
		variant = "model.onnx"
		add(variant, "synthetic-not-real-weights", true)
		add("voices.bin", "synthetic-voices", true)
		add("tokens.txt", "token 0", false)
		add("LICENSE", "Apache License\nVersion 2.0", false)
		for _, p := range []string{"phondata", "phonindex", "phontab", "en_dict", "lang/gmw/en"} {
			add("espeak-ng-data/"+p, "synthetic "+p, false)
		}
	}
	hf := &HuggingFaceClient{baseURL: "https://huggingface.co/api", client: &http.Client{Transport: packageFixtureTransport(f.roundTrip)}}
	return f, hf, variant
}
func (f *packageHubFixture) add(p string, b []byte, weight bool) {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	file := hubPackageFile{Path: p, Type: "file", Size: int64(len(b)), OID: hex.EncodeToString(h.Sum(nil))}
	if weight {
		d := sha256.Sum256(b)
		file.LFS = &struct {
			OID  string `json:"oid"`
			Size int64  `json:"size"`
		}{hex.EncodeToString(d[:]), int64(len(b))}
	}
	f.files = append(f.files, file)
	f.data[p] = b
}
func (f *packageHubFixture) roundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.fetches = append(f.fetches, r.URL.String())
	f.mu.Unlock()
	if r.Header.Get("Authorization") != "" || r.URL.User != nil {
		f.t.Fatal("credential leak")
	}
	var body []byte
	header := http.Header{}
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/models/fixture/speech/tree/"):
		body, _ = json.Marshal(f.files)
		if f.link != "" {
			header.Set("Link", f.link)
		}
	case strings.HasPrefix(r.URL.Path, "/api/models/fixture/speech"):
		body, _ = json.Marshal(f.meta)
	case strings.HasPrefix(r.URL.Path, "/fixture/speech/resolve/"+fixtureRevision+"/"):
		p := strings.TrimPrefix(r.URL.Path, "/fixture/speech/resolve/"+fixtureRevision+"/")
		for _, file := range f.files {
			if file.Path == p && file.LFS != nil {
				f.t.Fatal("resolution downloaded weights", p)
			}
		}
		body = f.data[p]
	default:
		f.t.Fatalf("unexpected source request: %s", r.URL.String())
	}
	return &http.Response{StatusCode: 200, Header: header, ContentLength: int64(len(body)), Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
}
func TestResolveCompleteSpeechRecipesWithoutDownloadingWeights(t *testing.T) {
	for _, arch := range []string{"whisper", "piper", "zipformer-streaming", "kokoro"} {
		t.Run(arch, func(t *testing.T) {
			f, hf, variant := newPackageHubFixture(t, arch)
			d, err := hf.DiscoverPackages(context.Background(), "fixture/speech")
			if err != nil {
				t.Fatal(err)
			}
			if d.Revision != fixtureRevision || len(d.Choices) != 1 || !d.Choices[0].Supported {
				t.Fatalf("discovery: %+v", d)
			}
			r, err := hf.ResolvePackage(context.Background(), PackageResolutionRequest{Repository: d.Repository, Revision: d.Revision, Variant: variant, Architecture: arch})
			if err != nil {
				t.Fatal(err)
			}
			if err = r.Manifest.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(r.Manifest.Artifacts) != len(f.files) {
				t.Fatal("dependency omitted")
			}
			for _, a := range r.Manifest.Artifacts {
				if a.Source.Revision != fixtureRevision || a.SHA256 == "" {
					t.Fatal("source not pinned")
				}
			}
		})
	}
}

func TestQwenRecipesRecognizePinnedDeclarativeLayouts(t *testing.T) {
	files := []hubPackageFile{
		{Path: "config.json", Type: "file", Size: 10},
		{Path: "preprocessor_config.json", Type: "file", Size: 10},
		{Path: "model-00001-of-00002.safetensors", Type: "file", Size: 10},
		{Path: "model-00002-of-00002.safetensors", Type: "file", Size: 10},
	}
	choices := packageChoices("Qwen/Qwen3-ASR-0.6B", files)
	if len(choices) != 1 || choices[0].Architecture != "qwen3-asr" || !choices[0].Supported {
		t.Fatalf("qwen asr choice: %+v", choices)
	}
	choices = packageChoices("Qwen/Qwen3-TTS-12Hz-0.6B-Base", append(files[:1], files[3:]...))
	if len(choices) != 1 || choices[0].Architecture != "qwen3-tts" || !choices[0].Supported {
		t.Fatalf("qwen tts choice: %+v", choices)
	}
	if got := packageChoices("community/qwen3-asr-gguf", files); len(got) != 0 {
		t.Fatalf("community conversion was implicitly trusted: %+v", got)
	}
}

func TestDiscoveryExplainsUnreviewedSpeechLayouts(t *testing.T) {
	choices := packageChoices("nvidia/canary-qwen-2.5b", []hubPackageFile{{Path: "model.safetensors", Type: "file", Size: 10}})
	if len(choices) != 0 {
		t.Fatalf("unreviewed repository became installable: %+v", choices)
	}
	// DiscoverPackages adds the user-facing no_reviewed_adapter explanation
	// after it has validated the complete immutable repository listing.
}
func TestSpeechResolutionRejectsIncompleteOrChangedSources(t *testing.T) {
	for _, name := range []string{"missing config", "missing license", "missing phonemizer", "changed revision", "changed repository", "truncated metadata", "changed data", "no weight digest", "case collision", "bad path", "pagination escape", "pagination loop", "gated", "private", "unknown variant"} {
		t.Run(name, func(t *testing.T) {
			arch := "piper"
			if name == "missing phonemizer" {
				arch = "kokoro"
			}
			f, hf, variant := newPackageHubFixture(t, arch)
			switch name {
			case "missing config":
				f.files = f.files[:1]
			case "missing license":
				f.files = f.files[:2]
			case "missing phonemizer":
				f.files = f.files[:4]
			case "changed revision":
				f.meta.SHA = strings.Repeat("b", 40)
			case "changed repository":
				f.meta.ID = "another/repo"
			case "truncated metadata":
				hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":`))}, nil
				})
			case "changed data":
				f.data[variant+".json"] = []byte(strings.Repeat(" ", int(f.files[1].Size)))
			case "no weight digest":
				f.files[0].LFS = nil
			case "case collision":
				f.add(strings.ToUpper(variant), []byte("duplicate"), true)
			case "bad path":
				f.files[0].Path = "../unsafe.onnx"
			case "pagination escape":
				f.link = `<https://huggingface.co/api/models/other/repo/tree/` + fixtureRevision + `?recursive=true>; rel="next"`
			case "pagination loop":
				f.link = `<https://huggingface.co/api/models/fixture/speech/tree/` + fixtureRevision + `?recursive=true&limit=1000>; rel="next"`
			case "gated":
				f.meta.Gated = true
			case "private":
				f.meta.Private = true
			case "unknown variant":
				variant = "other.onnx"
			}
			_, err := hf.ResolvePackage(context.Background(), PackageResolutionRequest{Repository: "fixture/speech", Revision: fixtureRevision, Variant: variant, Architecture: arch})
			if err == nil {
				t.Fatal("unsafe or incomplete package accepted")
			}
			var safe *ModelAcquisitionError
			if !errors.As(err, &safe) {
				t.Fatal("uncoded error", err)
			}
		})
	}
}
func TestSpeechSourceURLAndRedirectPolicy(t *testing.T) {
	_, hf, _ := newPackageHubFixture(t, "whisper")
	for _, address := range []string{"http://huggingface.co/repo", "https://huggingface.co.evil.test/repo", "https://user:secret@huggingface.co/repo", "https://127.0.0.1/model", "https://huggingface.co:8443/model", "https://example.com/model", "https://huggingface.co/model#secret"} {
		_, _, err := hf.packageGet(context.Background(), address, 1024, false)
		if err == nil {
			t.Fatal("unsafe URL accepted", address)
		}
	}
	u, _ := url.Parse("https://cas-bridge.xethub.hf.co/data?signature=opaque")
	req := &http.Request{URL: u, Header: http.Header{"Authorization": []string{"Bearer secret"}, "Cookie": []string{"session=secret"}}}
	if err := hf.packageClient(false).CheckRedirect(req, []*http.Request{{}}); err != nil {
		t.Fatal(err)
	}
	if len(req.Header) != 0 {
		t.Fatal("forwarded credentials")
	}
	if err := hf.packageClient(true).CheckRedirect(req, []*http.Request{{}}); err == nil {
		t.Fatal("metadata left HF")
	}
}
func TestSpeechSearchDoesNotUseGGUFOrFetchWeights(t *testing.T) {
	for _, category := range []ModelCategory{CategoryRecognition, CategoryGeneration} {
		_, hf, _ := newPackageHubFixture(t, "whisper")
		hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Query().Get("filter") == "gguf" || r.URL.Query().Get("pipeline_tag") == "" || r.URL.Path != "/api/models" {
				t.Fatal("incorrect speech discovery query", r.URL.String())
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"id":"fixture/speech","pipeline_tag":"text-to-speech"}]`))}, nil
		})
		items, err := hf.SearchModelsContext(context.Background(), SearchFilter{Category: category, Query: "speech", OnlyGGUF: true, Limit: 10})
		if err != nil || len(items) != 1 {
			t.Fatalf("speech search: %v, %v", items, err)
		}
	}
}
