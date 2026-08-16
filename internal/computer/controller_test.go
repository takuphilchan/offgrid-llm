package computer

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

type testDriver struct{ capture []byte }

func (d testDriver) Available() bool { return true }
func (d testDriver) Execute(_ context.Context, action Action) (DriverResult, error) {
	if action.Kind == Capture {
		return DriverResult{Capture: d.capture, MediaType: "image/png"}, nil
	}
	return DriverResult{Message: "ok"}, nil
}

func TestComputerUseRequiresApprovalScopeAndRedactsCapture(t *testing.T) {
	imageData := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			imageData.Set(x, y, color.White)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imageData); err != nil {
		t.Fatal(err)
	}
	store, err := artifacts.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	controller := NewController(testDriver{capture: encoded.Bytes()}, capabilities.NewBroker(capabilities.DefaultPolicy{}), store)
	scope := Scope{Target: "browser:work", MaxActions: 2, Redactions: []Rect{{X: 0, Y: 0, Width: 2, Height: 2}}}
	if _, err := controller.Begin(context.Background(), "admin", scope, false); err == nil {
		t.Fatal("computer-use session started without explicit approval")
	}
	session, err := controller.Begin(context.Background(), "admin", scope, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(context.Background(), "admin", Action{SessionID: session.ID, Kind: Click, Target: "browser:other"}); err == nil {
		t.Fatal("out-of-scope action was accepted")
	}
	result, err := controller.Execute(context.Background(), "admin", Action{SessionID: session.ID, Kind: Capture, Target: scope.Target})
	if err != nil {
		t.Fatal(err)
	}
	if result.Artifact == nil {
		t.Fatal("capture was not stored as an artifact")
	}
	reader, _, err := store.Open(result.Artifact.Digest)
	if err != nil {
		t.Fatal(err)
	}
	redacted, err := png.Decode(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	red, green, blue, _ := redacted.At(0, 0).RGBA()
	if red != 0 || green != 0 || blue != 0 {
		t.Fatal("configured capture region was not redacted")
	}
}

func TestEmergencyStopInvalidatesSessions(t *testing.T) {
	controller := NewController(testDriver{}, capabilities.NewBroker(capabilities.DefaultPolicy{}), nil)
	session, err := controller.Begin(context.Background(), "admin", Scope{Target: "browser:work", MaxActions: 3}, true)
	if err != nil {
		t.Fatal(err)
	}
	controller.EmergencyStop("admin")
	if _, err := controller.Execute(context.Background(), "admin", Action{SessionID: session.ID, Kind: Click, Target: "browser:work"}); err == nil {
		t.Fatal("action executed after emergency stop")
	}
	if err := controller.Reset(context.Background(), "admin", false); err == nil {
		t.Fatal("emergency stop reset without approval")
	}
	if err := controller.Reset(context.Background(), "admin", true); err != nil {
		t.Fatal(err)
	}
}
