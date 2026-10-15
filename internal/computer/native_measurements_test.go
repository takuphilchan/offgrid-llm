package computer

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNativeMeasurementsClosedPrivateAndConcurrent(t *testing.T) {
	m := &NativeMeasurements{}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); m.Add("input", time.Millisecond) }()
	}
	wg.Wait()
	m.Add("private document text", time.Second)
	m.Add("input", -time.Second)
	r := m.Report()
	if r.Stages["input"].Calls != 10 || r.Stages["input"].Milliseconds != 10 || r.Stages["readiness"] != nil || r.ModelCalls != nil || r.PromptTokens != nil {
		t.Fatal(r)
	}
	tokens := 123
	m.RecordModel(2, &tokens)
	tokens = 0
	r = m.Report()
	*r.PromptTokens = 5
	r.Stages["input"].Calls = 0
	if *m.Report().PromptTokens != 123 || m.Report().Stages["input"].Calls != 10 {
		t.Fatal("report exposed mutable state")
	}
	raw, _ := json.Marshal(m.Report())
	if strings.Contains(string(raw), "private") {
		t.Fatal("private labels serialized")
	}
	m.RecordModel(3, nil)
	if m.Report().PromptTokens != nil {
		t.Fatal("unavailable measurement fabricated")
	}
}

func TestDocumentExecutorMeasuresInputAndVerificationSeparately(t *testing.T) {
	e, _, _, action := documentDispatchSetup(t, "insert_text")
	m := &NativeMeasurements{}
	e.SetMeasurements(m)
	if _, err := e.Dispatch(context.Background(), action); err != nil {
		t.Fatal(err)
	}
	r := m.Report()
	if r.Stages["input"] == nil || r.Stages["verification"] == nil || r.Stages["input"].Calls != 1 || r.Stages["model"] != nil || r.Stages["readiness"] != nil {
		t.Fatal(r)
	}
}

func TestNativeMeasurementsKeepDiscoveryAndReadinessSeparate(t *testing.T) {
	c, _ := desktopControllerFixture(t)
	m := &NativeMeasurements{}
	c.SetMeasurements(m)
	if _, err := c.Applications(); err != nil {
		t.Fatal(err)
	}
	if r, err := c.VerifyApplication(context.Background(), "editor"); err != nil || !r.Verified {
		t.Fatal(r, err)
	}
	report := m.Report()
	if report.Stages["discovery"] == nil || report.Stages["readiness"] == nil || report.Stages["input"] != nil || report.Stages["model"] != nil {
		t.Fatal("discovery/readiness confused with input or model work", report)
	}
}
