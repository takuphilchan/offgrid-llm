package computer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Writable  bool   `json:"writable"`
	Invokable bool   `json:"invokable"`
}
type NativeView struct {
	Observation ControlObservation `json:"observation"`
	Elements    []NativeElement    `json:"elements"`
	Limited     bool               `json:"limited"`
}
