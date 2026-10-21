package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestExecuteDoctorUsesInjectedObservation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := testEnvironment(&stdout, &stderr, &fakeRunner{})
	called := 0
	env.Doctor = func(context.Context) (DoctorSnapshot, error) {
		called++
		return DoctorSnapshot{OS: "test", Arch: "amd64", CPUCores: 4, RAMBytes: 4096, ModelsDir: "fixture-models", StoragePresent: true, ModelCount: 1, ServerPort: 11611}, nil
	}
	if code := Execute(context.Background(), []string{"diagnose"}, env); code != 0 || called != 1 {
		t.Fatalf("code=%d called=%d stderr=%q", code, called, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fixture-models (present; writability not tested)") || !strings.Contains(stdout.String(), "1 model(s) installed") || !strings.Contains(stdout.String(), "Offline mode") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}
