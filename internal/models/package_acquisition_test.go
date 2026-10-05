package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testAcquisitionID = "model-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testRepairID = "model-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func networkPackageFixture(t *testing.T, arch string) (ModelPackageManifest, *HuggingFaceClient, *atomic.Int64) {
	t.Helper()
	m := testPackageManifest(arch)
	for i := range m.Artifacts {
		m.Artifacts[i].Source = &ArtifactSource{Repository: "https://huggingface.co/fixture/speech", Revision: fixtureRevision, Path: m.Artifacts[i].Path}
	}
	calls := &atomic.Int64{}
	hf := NewHuggingFaceClient()
	hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		for _, a := range m.Artifacts {
			if strings.HasSuffix(r.URL.Path, "/"+a.Path) {
				body := a.Role
				start := 0
				status := 200
				headers := http.Header{}
				if value := r.Header.Get("Range"); value != "" {
					fmt.Sscanf(value, "bytes=%d-", &start)
					status = 206
					headers.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(body)-1, len(body)))
				}
				if start >= len(body) {
					return &http.Response{StatusCode: 416, Header: http.Header{"Content-Range": []string{fmt.Sprintf("bytes */%d", len(body))}}, Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				return &http.Response{StatusCode: status, ContentLength: int64(len(body) - start), Header: headers, Body: io.NopCloser(strings.NewReader(body[start:]))}, nil
			}
		}
		return nil, fmt.Errorf("unexpected artifact")
	})
	return m, hf, calls
}
func noCheckpoint(PackageCheckpoint) error { return nil }
func fixtureProvenance() ModelProvenance {
	return ModelProvenance{Kind: "repository_metadata", Repository: "fixture/speech", Revision: fixtureRevision}
}

func TestAcquireSpeechPackagePublishesOnceWithReceipt(t *testing.T) {
	for _, arch := range []string{"whisper", "piper", "zipformer-streaming", "kokoro"} {
		t.Run(arch, func(t *testing.T) {
			m, hf, calls := networkPackageFixture(t, arch)
			s := NewPackageStore(t.TempDir())
			activated := false
			checkpoint := func(p PackageCheckpoint) error {
				if p.Phase == "activating" {
					items, err := s.List()
					if err != nil || len(items) != 0 {
						t.Fatal("partial package visible", items, err)
					}
					activated = true
				}
				return nil
			}
			state, err := s.AcquireFromHub(context.Background(), m, testAcquisitionID, false, fixtureProvenance(), hf, checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			if !activated || state.Integrity != "checked" || state.Provenance != "repository_metadata" || state.RuntimeCompatible {
				t.Fatalf("bad installed state: %+v", state)
			}
			if _, err = s.AcquireFromHub(context.Background(), m, testAcquisitionID, false, fixtureProvenance(), hf, noCheckpoint); err != nil {
				t.Fatal("lost acknowledgement not reconciled", err)
			}
			if calls.Load() != int64(len(m.Artifacts)) {
				t.Fatal("duplicate network acquisition")
			}
			if err = s.SettleAcquisition(m, testAcquisitionID, false); err != nil {
				t.Fatal(err)
			}
			if bytes, err := s.RetainedAcquisitionBytes(testAcquisitionID); err != nil || bytes != 0 {
				t.Fatal("staging not settled", bytes, err)
			}
		})
	}
}

func TestAcquirePauseResumeAndDiscardPreservesOtherPackages(t *testing.T) {
	m, hf, calls := networkPackageFixture(t, "piper")
	s := NewPackageStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	_, err := s.AcquireFromHub(ctx, m, testAcquisitionID, false, fixtureProvenance(), hf, func(p PackageCheckpoint) error {
		if p.Verified {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancel", err)
	}
	if items, _ := s.List(); len(items) != 0 {
		t.Fatal("partial package installed")
	}
	bytes, err := s.RetainedAcquisitionBytes(testAcquisitionID)
	if err != nil || bytes <= 0 {
		t.Fatal("cancelled data missing", bytes, err)
	}
	if _, err = s.AcquireFromHub(context.Background(), m, testAcquisitionID, false, fixtureProvenance(), hf, noCheckpoint); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != int64(len(m.Artifacts)) {
		t.Fatal("verified artifact redownloaded")
	}
	other := testPackageManifest("whisper")
	if _, err = s.Import(context.Background(), other, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	if err = s.SettleAcquisition(m, testAcquisitionID, true); err != nil {
		t.Fatal(err)
	}
	if items, _ := s.List(); len(items) != 2 {
		t.Fatal("cleanup removed installed data")
	}
}

func TestPackageTransferIntegrityAndRangeResponses(t *testing.T) {
	for _, mode := range []string{"200", "206", "416", "corrupt", "short", "oversized", "cancel", "bad-range"} {
		t.Run(mode, func(t *testing.T) {
			m, hf, _ := networkPackageFixture(t, "whisper")
			s := NewPackageStore(t.TempDir())
			root, err := s.open()
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			a := m.Artifacts[0]
			partial := "partial"
			seed := "we"
			if err = root.WriteFile(partial, []byte(seed), 0600); err != nil {
				t.Fatal(err)
			}
			hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
				status, body, header := 200, "weights", http.Header{}
				switch mode {
				case "206":
					status = 206
					body = "ights"
					header.Set("Content-Range", "bytes 2-6/7")
				case "bad-range":
					status = 206
					body = "weights"
					header.Set("Content-Range", "bytes 1-7/8")
				case "416":
					status = 416
					body = ""
					header.Set("Content-Range", "bytes */7")
				case "corrupt":
					body = "WRONG!!"
				case "short":
					body = "weight"
				case "oversized":
					body = "weights-extra"
				case "cancel":
					return nil, context.Canceled
				}
				return &http.Response{StatusCode: status, ContentLength: int64(len(body)), Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			err = hf.transferPackageArtifact(context.Background(), root, partial, a, func(int64) error { return nil })
			if mode == "200" || mode == "206" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("bad transfer accepted")
			}
		})
	}
}

func TestPackagePublicationCrashRecoveryAndRepair(t *testing.T) {
	for _, phase := range []string{"intent", "previous-renamed", "activated", "failed-checkpoint"} {
		t.Run(phase, func(t *testing.T) {
			m, hf, calls := networkPackageFixture(t, "piper")
			s := NewPackageStore(t.TempDir())
			ctx := context.Background()
			if _, err := s.Import(ctx, m, fixturePackageSource); err != nil {
				t.Fatal(err)
			}
			before := calls.Load()
			var interrupted = errors.New("injected process loss")
			_, err := s.AcquireFromHub(ctx, m, testRepairID, true, fixtureProvenance(), hf, func(p PackageCheckpoint) error {
				if p.Phase == "activating" && phase != "activated" {
					return interrupted
				}
				return nil
			})
			if phase != "activated" && !errors.Is(err, interrupted) {
				t.Fatal("fault not injected", err)
			}
			if phase == "activated" && err != nil {
				t.Fatal(err)
			}
			root, err := s.open()
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			key, _ := packageKey(m.ID, m.Revision)
			if phase == "intent" || phase == "previous-renamed" {
				j := packagePublication{SchemaVersion: 1, OperationID: testRepairID, ID: m.ID, Revision: m.Revision, ManifestDigest: ManifestDigest(m), Repair: true}
				if err = writePackageJSON(root, ".publish-"+testRepairID+".json", j); err != nil {
					t.Fatal(err)
				}
				if phase == "previous-renamed" {
					if err = root.Rename(key, ".previous-"+testRepairID); err != nil {
						t.Fatal(err)
					}
				}
			}
			restarted := NewPackageStore(s.modelsDir)
			done, err := restarted.Reconcile(ctx, m, testRepairID)
			if err != nil {
				t.Fatal(err)
			}
			if done != (phase == "activated") {
				t.Fatal("incorrect receipt recovery", done)
			}
			if _, err = restarted.Verify(ctx, m.ID, m.Revision); err != nil {
				t.Fatal("working original not recoverable", err)
			}
			if _, err = restarted.AcquireFromHub(ctx, m, testRepairID, true, fixtureProvenance(), hf, noCheckpoint); err != nil {
				t.Fatal(err)
			}
			if calls.Load()-before != int64(len(m.Artifacts)) {
				t.Fatal("recovery downloaded twice")
			}
		})
	}
}

func TestAcquisitionBlocksRemovalButNotOtherModelInventory(t *testing.T) {
	m, hf, _ := networkPackageFixture(t, "whisper")
	s := NewPackageStore(t.TempDir())
	entered, release := make(chan struct{}), make(chan struct{})
	hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 200, ContentLength: 7, Body: io.NopCloser(strings.NewReader("weights"))}, nil
	})
	done := make(chan error, 1)
	go func() {
		_, err := s.AcquireFromHub(context.Background(), m, testAcquisitionID, false, fixtureProvenance(), hf, noCheckpoint)
		done <- err
	}()
	<-entered
	if err := s.Remove(m.ID, m.Revision); !errors.Is(err, ErrPackageInUse) {
		t.Fatal("removal overtook acquisition", err)
	}
	listed := make(chan error, 1)
	go func() { _, err := s.List(); listed <- err }()
	select {
	case err := <-listed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("network blocked global inventory")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestNetworkCancellationClosesActualHTTPBody(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "7")
		w.WriteHeader(200)
		w.Write([]byte("w"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(finished)
	}))
	defer server.Close()
	m, hf, _ := networkPackageFixture(t, "whisper")
	transport := server.Client().Transport
	hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
		request := r.Clone(r.Context())
		u := *r.URL
		request.URL = &u
		request.URL.Scheme = "http"
		request.URL.Host = strings.TrimPrefix(server.URL, "http://")
		return transport.RoundTrip(request)
	})
	s := NewPackageStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := s.AcquireFromHub(ctx, m, testAcquisitionID, false, fixtureProvenance(), hf, noCheckpoint)
		done <- err
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not settle")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("HTTP stream remained open")
	}
}

func TestReconcileRejectsAlteredManifestAndStagingLinks(t *testing.T) {
	m, hf, _ := networkPackageFixture(t, "whisper")
	s := NewPackageStore(t.TempDir())
	ctx := context.Background()
	if _, err := s.AcquireFromHub(ctx, m, testAcquisitionID, false, fixtureProvenance(), hf, noCheckpoint); err != nil {
		t.Fatal(err)
	}
	altered := m
	altered.Name = "changed after commit"
	b, _ := json.Marshal(altered)
	p := filepath.Join(s.modelsDir, PackageDirectory, m.ID, m.Revision, "manifest.json")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reconcile(ctx, m, testAcquisitionID); err == nil {
		t.Fatal("altered manifest reconciled")
	}
	if _, err := s.Verify(ctx, m.ID, m.Revision); err == nil {
		t.Fatal("altered receipt verified")
	}
}

func TestAcquisitionDiscardRejectsLinkedStagingAndPreservesUnrelatedFiles(t *testing.T) {
	m, _, _ := networkPackageFixture(t, "whisper")
	s := NewPackageStore(t.TempDir())
	root, err := s.open()
	if err != nil {
		t.Fatal(err)
	}
	root.Close()
	outside := t.TempDir()
	protected := filepath.Join(outside, "document.txt")
	if err := os.WriteFile(protected, []byte("user document"), 0600); err != nil {
		t.Fatal(err)
	}
	stage, _ := acquisitionStage(testAcquisitionID)
	link := filepath.Join(s.modelsDir, PackageDirectory, stage)
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink creation unavailable:", err)
	}
	if err := s.SettleAcquisition(m, testAcquisitionID, true); err == nil {
		t.Fatal("linked staging accepted")
	}
	if b, err := os.ReadFile(protected); err != nil || string(b) != "user document" {
		t.Fatal("external document changed", err)
	}
	if _, err := s.RetainedAcquisitionBytes(testAcquisitionID); err == nil {
		t.Fatal("linked staging was counted as safe data")
	}
}

func TestSlowPackageDoesNotBlockLegacyDownload(t *testing.T) {
	m, hf, _ := networkPackageFixture(t, "whisper")
	s := NewPackageStore(t.TempDir())
	entered, release := make(chan struct{}), make(chan struct{})
	hf.client.Transport = packageFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/fixture/speech/") {
			close(entered)
			<-release
		}
		return &http.Response{StatusCode: 200, ContentLength: 7, Body: io.NopCloser(strings.NewReader("weights"))}, nil
	})
	done := make(chan error, 1)
	go func() {
		_, err := s.AcquireFromHub(context.Background(), m, testAcquisitionID, false, fixtureProvenance(), hf, noCheckpoint)
		done <- err
	}()
	<-entered
	f, err := os.CreateTemp(t.TempDir(), "legacy-*.part")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	response, err := openResumableOffsetContext(context.Background(), hf.client, "https://huggingface.co/fixture/legacy/resolve/main/model.gguf", "test", 0, func() error { return nil })
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	_, err = io.Copy(f, response.response.Body)
	response.response.Body.Close()
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
