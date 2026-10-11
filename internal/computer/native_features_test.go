package computer

import (
	"context"
	"testing"
)

type documentFeatureFixture struct {
	NativeAdapter
	NativeDocumentBackend
	version int
}

func (d documentFeatureFixture) DocumentContractVersion() int { return d.version }

type documentVersionOnlyFixture struct{ NativeAdapter }

func (documentVersionOnlyFixture) DocumentContractVersion() int { return 1 }

func TestNativeDocumentFeatureRequiresImplementedKnownContract(t *testing.T) {
	for _, v := range []int{0, 1, 2} {
		features := ProviderNativeFeatures(documentFeatureFixture{version: v})
		if (features.Documents == 1) != (v == 1) {
			t.Fatal(features)
		}
	}
	if ProviderNativeFeatures(nil).Documents != 0 {
		t.Fatal("legacy adapter advertised documents")
	}
	if ProviderNativeFeatures(documentVersionOnlyFixture{}).Documents != 0 {
		t.Fatal("version flag without execution backend advertised documents")
	}
	for _, raw := range []string{`{"native_documents":null}`, `{"native_documents":0}`, `{"native_documents":2}`, `{"native_documents":1,"shell":1}`, `{"native_documents":"1"}`, `null`} {
		var features NativeFeatures
		if DecodeControlMessage([]byte(raw), &features) == nil && features.Validate() == nil {
			t.Fatal("accepted", raw)
		}
	}
}

func TestNativeDocumentNegotiationDoesNotExpandGrantOrRevealOtherActor(t *testing.T) {
	for _, version := range []int{0, 1} {
		h := NewBrowserHub()
		code, _ := h.PairCode("alice")
		target := TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
		session, _, err := h.PairNativeFeatures(code, "Owned editor", target, 2, ApprovalFullTask, NativeFeatures{Documents: version})
		if err != nil {
			t.Fatal(err)
		}
		got, err := h.NativeFeatures("alice", session.ID)
		if err != nil || got.Documents != version || session.Desktop != nil || session.Target == nil || *session.Target != target {
			t.Fatal("grant changed", got, err)
		}
		if _, err = h.NativeFeatures("bob", session.ID); err == nil {
			t.Fatal("features leaked across actors")
		}
		if _, err = h.Execute(context.Background(), "alice", session.ID, "run", "call", "computer_open_application", []byte(`{}`)); err == nil {
			t.Fatal("selected-window grant widened")
		}
		// Contract negotiation alone cannot activate tools without dispatch support.
		if _, err = h.Execute(context.Background(), "alice", session.ID, "run", "call", "computer_documents", []byte(`{}`)); err == nil {
			t.Fatal("unimplemented document tool advertised")
		}
	}
}
