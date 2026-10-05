package models

import (
	"errors"
	"testing"
)

func acquisitionFixture(t *testing.T) ModelPackageManifest {
	t.Helper()
	m := testPackageManifest("piper")
	for i := range m.Artifacts {
		m.Artifacts[i].Source = &ArtifactSource{Repository: "https://huggingface.co/fixture/speech", Revision: fixtureRevision, Path: m.Artifacts[i].Path}
	}
	return m
}
func TestPackagePreflightSpaceAndRuntimeAreIndependent(t *testing.T) {
	m := acquisitionFixture(t)
	p, err := PreflightPackage(m, 1<<30, 512)
	if err != nil {
		t.Fatal(err)
	}
	if p.RequiredFreeBytes != 2*p.TransferBytes+512+(16<<20) || p.RuntimeAvailable || p.Readiness.RuntimeCompatible || len(p.Warnings) == 0 {
		t.Fatalf("bad preflight: %+v", p)
	}
	_, err = PreflightPackage(m, p.RequiredFreeBytes-1, 512)
	var safe *ModelAcquisitionError
	if !errors.As(err, &safe) || safe.Code != "insufficient_space" {
		t.Fatal("space not enforced", err)
	}
	if _, err = PreflightPackage(m, p.RequiredFreeBytes, 512); err != nil {
		t.Fatal(err)
	}
}
func TestPackagePreflightRejectsUnverifiableAcquisition(t *testing.T) {
	for _, name := range []string{"no source", "query", "credentials", "port", "host", "traversal", "revision", "missing dependency", "digest"} {
		t.Run(name, func(t *testing.T) {
			m := acquisitionFixture(t)
			switch name {
			case "no source":
				m.Artifacts[0].Source = nil
			case "query":
				m.Artifacts[0].Source.Repository += "?token=secret"
			case "credentials":
				m.Artifacts[0].Source.Repository = "https://user:secret@huggingface.co/fixture/speech"
			case "port":
				m.Artifacts[0].Source.Repository = "https://huggingface.co:8080/fixture/speech"
			case "host":
				m.Artifacts[0].Source.Repository = "https://127.0.0.1/fixture/speech"
			case "traversal":
				m.Artifacts[0].Source.Path = "../weights.onnx"
			case "revision":
				m.Artifacts[0].Source.Revision = "main"
			case "missing dependency":
				m.Artifacts = m.Artifacts[:1]
			case "digest":
				m.Artifacts[0].SHA256 = ""
			}
			if _, err := PreflightPackage(m, 1<<30, 0); err == nil {
				t.Fatal("unsafe package accepted")
			}
		})
	}
}
