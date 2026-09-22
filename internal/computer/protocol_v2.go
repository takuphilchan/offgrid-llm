package computer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"
)

// ControlProtocolVersion is the next driver contract, NOT an advertised or
// installed native capability. ProtocolVersion remains the browser-v1 adapter.
const ControlProtocolVersion = 2
const MaxStepActions = 10
const MaxIPCFrame = 1 << 20

var (
	ErrInvalidControl   = errors.New("computer_invalid_action")
	ErrControlScope     = errors.New("computer_scope_violation")
	ErrControlApproval  = errors.New("computer_approval_invalid")
	ErrStaleObservation = errors.New("computer_stale_observation")
	ErrProhibitedAction = errors.New("computer_prohibited_action")
)

// TargetIdentity is issued by the host, never inferred from a window title,
// executable name, URL, PID alone, or model-provided selector.
type TargetIdentity struct {
	ID                string `json:"id"`
	OSSession         string `json:"os_session"`
	ProcessGeneration string `json:"process_generation"`
	Surface           string `json:"surface"`
	Driver            string `json:"driver"`
}

func opaqueID(value string) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == ':') {
			return false
		}
	}
	return true
}
func (target TargetIdentity) valid() bool {
	if !opaqueID(target.ID) || !opaqueID(target.OSSession) || !opaqueID(target.ProcessGeneration) || !opaqueID(target.Surface) {
		return false
	}
	switch target.Driver {
	case "browser", "windows-uia", "macos-accessibility", "linux-atspi":
		return true
	}
	return false
}

func (target TargetIdentity) Validate() error {
	if !target.valid() {
		return ErrControlScope
	}
	return nil
}

type ControlBinding struct {
	Actor     string         `json:"actor"`
	Task      string         `json:"task"`
	Companion string         `json:"companion"`
	Session   string         `json:"session"`
	Target    TargetIdentity `json:"target"`
}

func (binding ControlBinding) valid() bool {
	return opaqueID(binding.Actor) && opaqueID(binding.Task) && opaqueID(binding.Companion) && opaqueID(binding.Session) && binding.Target.valid()
}

func (binding ControlBinding) Validate() error {
	if !binding.valid() {
		return ErrControlScope
	}
	return nil
}

// Operation is a closed typed union. No executable strings, arbitrary key
// chords, selectors, scripts, raw native calls or unrestricted paths exist here.
type Operation struct {
	Kind        string           `json:"kind"`
	Element     string           `json:"element,omitempty"`
	Text        *string          `json:"text,omitempty"`
	Option      string           `json:"option,omitempty"`
	Checked     *bool            `json:"checked,omitempty"`
	Shortcut    string           `json:"shortcut,omitempty"`
	Scroll      *ScrollOperation `json:"scroll,omitempty"`
	Destination string           `json:"destination,omitempty"`
	File        *FileOperation   `json:"file,omitempty"`
	Point       *SurfacePoint    `json:"point,omitempty"`
}
type ScrollOperation struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type FileOperation struct {
	FolderGrant      string `json:"folder_grant"`
	Source           string `json:"source,omitempty"` // host-issued file identity, not path
	DestinationGrant string `json:"destination_grant,omitempty"`
	Name             string `json:"name,omitempty"` // one new leaf component, not path
}
type SurfacePoint struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Image string `json:"image"`
}

// Validate uses an exact field set for each operation; adding irrelevant fields
// must not smuggle new authority into a seemingly read-only action.
func (op Operation) Validate() error {
	want := Operation{Kind: op.Kind}
	switch op.Kind {
	case "observe", "capture", "focus":
	case "activate":
		if !opaqueID(op.Element) {
			return ErrInvalidControl
		}
		want.Element = op.Element
	case "replace_text":
		if !opaqueID(op.Element) || op.Text == nil || len(*op.Text) > 16000 || !utf8.ValidString(*op.Text) || bytes.IndexByte([]byte(*op.Text), 0) >= 0 {
			return ErrInvalidControl
		}
		want.Element, want.Text = op.Element, op.Text
	case "select":
		if !opaqueID(op.Element) || !opaqueID(op.Option) {
			return ErrInvalidControl
		}
		want.Element, want.Option = op.Element, op.Option
	case "set_checked":
		if !opaqueID(op.Element) || op.Checked == nil {
			return ErrInvalidControl
		}
		want.Element, want.Checked = op.Element, op.Checked
	case "scroll":
		if op.Scroll == nil || op.Scroll.X < -2000 || op.Scroll.X > 2000 || op.Scroll.Y < -2000 || op.Scroll.Y > 2000 || op.Scroll.X == 0 && op.Scroll.Y == 0 {
			return ErrInvalidControl
		}
		want.Scroll = op.Scroll
	case "shortcut":
		if !opaqueID(op.Element) {
			return ErrInvalidControl
		}
		switch op.Shortcut {
		case "copy", "paste", "undo", "redo", "select_all", "save", "enter", "escape", "tab", "reverse_tab", "left", "right", "up", "down", "page_up", "page_down", "home", "end":
		default:
			return ErrInvalidControl
		}
		want.Element, want.Shortcut = op.Element, op.Shortcut
	case "navigate", "upload", "download":
		// Destination is a locally prepared origin/download grant, not an
		// arbitrary model URL. Upload additionally binds a host-issued file.
		if !opaqueID(op.Destination) {
			return ErrInvalidControl
		}
		want.Destination = op.Destination
		if op.Kind == "upload" {
			if op.File == nil || !opaqueID(op.File.Source) || !opaqueID(op.File.FolderGrant) || op.File.Name != "" || op.File.DestinationGrant != "" {
				return ErrInvalidControl
			}
			want.File = op.File
		}
	case "create_file", "copy_file", "rename_file", "move_file", "trash_file", "overwrite_file":
		if op.File == nil || !opaqueID(op.File.FolderGrant) {
			return ErrInvalidControl
		}
		file := *op.File
		if op.Kind != "create_file" && !opaqueID(file.Source) {
			return ErrInvalidControl
		}
		if op.Kind == "create_file" && file.Source != "" {
			return ErrInvalidControl
		}
		if op.Kind == "trash_file" {
			if file.Name != "" || file.DestinationGrant != "" {
				return ErrInvalidControl
			}
		} else {
			if file.Name == "" || len(file.Name) > 240 || file.Name == "." || file.Name == ".." || bytes.ContainsAny([]byte(file.Name), "/\\:\x00") {
				return ErrInvalidControl
			}
		}
		if op.Kind == "move_file" || op.Kind == "copy_file" {
			if !opaqueID(file.DestinationGrant) {
				return ErrInvalidControl
			}
		} else if file.DestinationGrant != "" {
			return ErrInvalidControl
		}
		want.File = op.File
	case "coordinate_activate":
		if op.Point == nil || op.Point.X < 0 || op.Point.Y < 0 || !opaqueID(op.Point.Image) {
			return ErrInvalidControl
		}
		want.Point = op.Point
	default:
		return ErrInvalidControl
	}
	actual, _ := json.Marshal(op)
	allowed, _ := json.Marshal(want)
	if !bytes.Equal(actual, allowed) {
		return ErrInvalidControl
	}
	return nil
}

// SeparateConfirmation is deliberately conservative and cannot be downgraded
// by a model-provided "safe" flag. A driver may require stronger confirmation.
func (op Operation) SeparateConfirmation() bool {
	switch op.Kind {
	case "activate", "coordinate_activate", "navigate", "upload", "download", "overwrite_file", "trash_file", "shortcut":
		return true
	}
	return false
}

type PreparedControlAction struct {
	ID              string      `json:"id"`
	Operation       Operation   `json:"operation"`
	ControlIdentity string      `json:"control_identity"`
	Precondition    string      `json:"precondition"`    // local driver-issued state digest
	ExpectedChange  string      `json:"expected_change"` // local prepared change digest
	ApprovalClass   ActionClass `json:"approval_class"`
}
type BoundedStep struct {
	ID      string                  `json:"id"`
	Binding ControlBinding          `json:"binding"`
	Actions []PreparedControlAction `json:"actions"`
}

func (step BoundedStep) Digest() (string, error) {
	if !opaqueID(step.ID) || !step.Binding.valid() || len(step.Actions) == 0 || len(step.Actions) > MaxStepActions {
		return "", ErrInvalidControl
	}
	seen := map[string]bool{}
	for _, action := range step.Actions {
		if !opaqueID(action.ID) || seen[action.ID] || action.Operation.Validate() != nil || !opaqueID(action.ControlIdentity) || !digestID(action.Precondition) || !digestID(action.ExpectedChange) || !action.ApprovalClass.Valid() {
			return "", ErrInvalidControl
		}
		if action.Operation.SeparateConfirmation() && len(step.Actions) != 1 {
			return "", ErrControlApproval
		}
		seen[action.ID] = true
	}
	data, err := json.Marshal(step)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
func digestID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

type StepGrant struct {
	ID         string         `json:"id"`
	ConsentID  string         `json:"consent_id"`
	Binding    ControlBinding `json:"binding"`
	StepDigest string         `json:"step_digest"`
	IssuedAt   time.Time      `json:"issued_at"`
	ExpiresAt  time.Time      `json:"expires_at"`
}

// LocalConsent is supplied by the host authority, not decoded from model/tool
// arguments. ID is renewed after restart, pause/takeover, and revocation.
type LocalConsent struct {
	ID           string
	Binding      ControlBinding
	ApprovalMode ApprovalMode
	IssuedAt     time.Time
	ExpiresAt    time.Time
	Active       bool
}

func (grant StepGrant) Validate(step BoundedStep, consent LocalConsent, now time.Time) error {
	digest, err := step.Digest()
	if err != nil {
		return err
	}
	if !consent.Active || !opaqueID(consent.ID) || grant.ConsentID != consent.ID || consent.Binding != step.Binding || grant.Binding != step.Binding {
		return ErrControlScope
	}
	if !opaqueID(grant.ID) || grant.StepDigest != digest || grant.IssuedAt.Before(consent.IssuedAt) || grant.IssuedAt.After(now) ||
		!now.Before(grant.ExpiresAt) || !grant.IssuedAt.Before(grant.ExpiresAt) || grant.ExpiresAt.After(grant.IssuedAt.Add(5*time.Minute)) || grant.ExpiresAt.After(consent.ExpiresAt) || !now.Before(consent.ExpiresAt) {
		return ErrControlApproval
	}
	return nil
}

type ControlObservation struct {
	ID          string         `json:"id"`
	Target      TargetIdentity `json:"target"`
	Sequence    uint64         `json:"sequence"`
	CapturedAt  time.Time      `json:"captured_at"`
	StateDigest string         `json:"state_digest"`
}

func (observation ControlObservation) Validate(target TargetIdentity, now time.Time) error {
	if !target.valid() || observation.Target != target {
		return ErrControlScope
	}
	if !opaqueID(observation.ID) || observation.Sequence == 0 || !digestID(observation.StateDigest) || observation.CapturedAt.After(now) || now.Sub(observation.CapturedAt) > 15*time.Second {
		return ErrStaleObservation
	}
	return nil
}

// DecodeControlMessage rejects unknown fields, oversized frames and trailing
// JSON. Private worker transports add framing; EOF is never an executable call.
func DecodeControlMessage(data []byte, destination any) error {
	if len(data) == 0 || len(data) > MaxIPCFrame || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return ErrInvalidControl
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return ErrInvalidControl
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return ErrInvalidControl
	}
	return nil
}

// Worker IPC uses a big-endian length prefix on private inherited pipes. It is
// never a TCP listener. Reject the length before allocating or reading a body.
func ReadControlFrame(reader io.Reader, destination any) error {
	var size uint32
	if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
		return err
	}
	if size == 0 || size > MaxIPCFrame {
		return ErrInvalidControl
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(reader, data); err != nil {
		return err
	}
	return DecodeControlMessage(data, destination)
}

func WriteControlFrame(writer io.Writer, message any) error {
	data, err := json.Marshal(message)
	if err != nil || len(data) == 0 || len(data) > MaxIPCFrame || bytes.Equal(data, []byte("null")) {
		return ErrInvalidControl
	}
	frame := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(frame, uint32(len(data)))
	copy(frame[4:], data)
	for len(frame) > 0 {
		n, err := writer.Write(frame)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(frame) {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}

type DriverAvailability struct {
	Installed, Permitted, Available, Qualified bool
	ReasonCode                                 string
}
type ControlResult struct {
	ActionID   string `json:"action_id"`
	Outcome    string `json:"outcome"`
	EvidenceID string `json:"evidence_id,omitempty"`
	Uncertain  bool   `json:"uncertain"`
}

// Implementations may never obtain dispatch authority merely by implementing
// this interface. The durable supervisor and local consent authority must call
// Prepare/Execute, journal intent, and enforce Stop independently of inference.
type ControlDriver interface {
	Availability(context.Context) (DriverAvailability, error)
	Targets(context.Context) ([]TargetIdentity, error)
	Observe(context.Context, TargetIdentity) (ControlObservation, error)
	Prepare(context.Context, ControlBinding, Operation, ControlObservation) (PreparedControlAction, error)
	Execute(context.Context, PreparedControlAction, ControlObservation) (ControlResult, error)
	Verify(context.Context, PreparedControlAction, ControlResult) (ControlResult, error)
	Cancel(context.Context) error
	Shutdown(context.Context) error
}
