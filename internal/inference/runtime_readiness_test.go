package inference

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type readinessTransport func(*http.Request) (*http.Response, error)

func (f readinessTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func isolatedReadinessManager(t *testing.T) *BinaryManager {
	t.Helper()
	t.Setenv("PATH", "")
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", "")
	return &BinaryManager{version: "b10516", binDir: t.TempDir()}
}

func TestRuntimeReadinessNeverDownloadsOnInspection(t *testing.T) {
	bm := isolatedReadinessManager(t)
	bm.downloadClient = &http.Client{Transport: readinessTransport(func(*http.Request) (*http.Response, error) { t.Fatal("inspection downloaded"); return nil, nil })}
	status := bm.RuntimeStatus()
	if status.State != "missing" || !status.DownloadRequired || status.DownloadBytes <= 0 || status.Mode != bm.runtimeMode() {
		t.Fatalf("unexpected status: %+v", status)
	}
	if entries, _ := os.ReadDir(bm.binDir); len(entries) != 0 {
		t.Fatal("inspection changed files")
	}
}

func TestRuntimePreparationVerifiesNativeProbeAndCancellation(t *testing.T) {
	for _, mode := range []string{"ready", "fail", "wait"} {
		t.Run(mode, func(t *testing.T) {
			bm := isolatedReadinessManager(t)
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("OFFGRID_LLAMA_SERVER_PATH", binary)
			t.Setenv("OFFGRID_TEST_READINESS_PROBE", mode)
			limit := 5 * time.Second
			if mode == "wait" {
				limit = 100 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), limit)
			defer cancel()
			_, err = bm.PrepareRuntime(ctx)
			status := bm.RuntimeStatus()
			switch mode {
			case "ready":
				if err != nil || status.State != "ready" || status.DownloadRequired {
					t.Fatalf("%+v: %v", status, err)
				}
				bm.SetGPUEnabled(true)
				if bm.RuntimeStatus().State == "ready" {
					t.Fatal("configuration change retained stale readiness")
				}
			case "fail":
				if err == nil || status.State != "failed" {
					t.Fatalf("failed help executable accepted: %+v, %v", status, err)
				}
			case "wait":
				if !errors.Is(err, context.DeadlineExceeded) || status.State != "cancelled" {
					t.Fatalf("probe ignored cancellation: %+v, %v", status, err)
				}
			}
		})
	}
}

func TestRuntimeReadinessDoesNotTrustRemovedBinary(t *testing.T) {
	bm := isolatedReadinessManager(t)
	file := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	bm.readyPath = file
	bm.readiness = RuntimeReadiness{State: "ready", Mode: bm.runtimeMode()}
	if bm.RuntimeStatus().State != "ready" {
		t.Fatal("fixture not ready")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if bm.RuntimeStatus().State == "ready" {
		t.Fatal("removed engine reported ready")
	}
}

func TestRuntimePreparationCorruptionAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{true: "cancelled", false: "corrupt"}[cancelled], func(t *testing.T) {
			bm := isolatedReadinessManager(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			bm.downloadClient = &http.Client{Transport: readinessTransport(func(r *http.Request) (*http.Response, error) {
				if cancelled {
					cancel()
					return nil, r.Context().Err()
				}
				return &http.Response{StatusCode: 200, ContentLength: 7, Body: io.NopCloser(strings.NewReader("corrupt")), Header: make(http.Header)}, nil
			})}
			if _, err := bm.PrepareRuntime(ctx); err == nil {
				t.Fatal("bad preparation succeeded")
			}
			status := bm.RuntimeStatus()
			want := "failed"
			if cancelled {
				want = "cancelled"
			}
			if status.State != want {
				t.Fatalf("status %+v", status)
			}
			if !cancelled && status.DownloadedBytes != 7 {
				t.Fatalf("progress missing: %+v", status)
			}
			if _, err := os.Stat(bm.versionedPath()); !os.IsNotExist(err) {
				t.Fatal("invalid runtime installed")
			}
			if entries, _ := os.ReadDir(bm.binDir); len(entries) != 0 {
				t.Fatal("failed preparation left staged files")
			}
		})
	}
}

func TestRuntimePreparationSingleFlightAndExplicitFailure(t *testing.T) {
	bm := isolatedReadinessManager(t)
	bm.prepareMu.Lock()
	_, err := bm.PrepareRuntime(context.Background())
	bm.prepareMu.Unlock()
	if !errors.Is(err, ErrRuntimePreparationBusy) {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", filepath.Join(t.TempDir(), "missing"))
	if _, err := bm.PrepareRuntime(context.Background()); err == nil {
		t.Fatal("missing explicit runtime accepted")
	}
	if bm.RuntimeStatus().State != "failed" {
		t.Fatal("failure not observable")
	}
}
