package computer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"unicode/utf8"
)

func nativeHash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func nativeID() string { return randomID() }

// NativeAdapter is the host provider contract used by the same supervisor,
// execution journal and private worker on every desktop platform. Providers do
// not grant permission; local consent and exact grants remain in the host core.
// Structured scope must fail closed if an OS cannot enforce it.
type NativeAdapter interface {
	Targets() ([]NativeTarget, error)
	Select(string) (NativeTarget, error)
	Observe(context.Context) (NativeView, error)
	Prepare(ControlBinding, Operation, ControlObservation) (PreparedControlAction, error)
	ApprovalSummary(BoundedStep) (string, error)
	FocusSelected() error
	Dispatch(context.Context, PreparedControlAction) (ControlResult, error)
	Close()
}

type NativeTarget struct {
	Identity TargetIdentity `json:"identity"`
	Title    string         `json:"title"`
}
type NativeElement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Writable    bool    `json:"writable"`
	Invokable   bool    `json:"invokable"`
	Checkable   bool    `json:"checkable"`
	Checked     *bool   `json:"checked,omitempty"`
	Text        *string `json:"text,omitempty"`
	TextLimited bool    `json:"text_limited,omitempty"`
}

// Text is exposed only after platform-specific protected-control checks. It is
// untrusted task data and is bounded independently from the local state digest.
func nativeText(value string) (*string, bool) {
	limited := len(value) > 2000
	if limited {
		value = value[:2000]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return &value, limited
}

type NativeVerifier interface {
	VerifyText(ControlBinding, string, string, ControlObservation) (bool, error)
	VerifyChecked(ControlBinding, string, bool, ControlObservation) (bool, error)
}

func BoundNativeView(view NativeView) NativeView {
	// Leave room for envelope and metadata in the service's 48 KiB reply bound.
	for len(view.Elements) > 0 {
		encoded, _ := json.Marshal(view)
		if len(encoded) <= 40<<10 {
			break
		}
		view.Elements = view.Elements[:len(view.Elements)-1]
		view.Limited = true
	}
	return view
}

type NativeView struct {
	Observation ControlObservation `json:"observation"`
	Elements    []NativeElement    `json:"elements"`
	Limited     bool               `json:"limited"`
}
