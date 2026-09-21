// Package computer provides the governed control plane for computer use. OS
// automation drivers plug into this boundary; policy, approval, redaction,
// limits, audit events, and emergency stop remain driver-independent.
package computer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

type ActionKind string

const (
	Capture ActionKind = "capture"
	Click   ActionKind = "click"
	Type    ActionKind = "type"
	Key     ActionKind = "key"
	Scroll  ActionKind = "scroll"
)

type Rect struct{ X, Y, Width, Height int }

type Scope struct {
	Target         string        `json:"target"`
	ExpiresIn      time.Duration `json:"-"`
	ExpiresSeconds int           `json:"expires_seconds,omitempty"`
	MaxActions     int           `json:"max_actions"`
	Redactions     []Rect        `json:"redactions,omitempty"`
}

type Session struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	Target     string    `json:"target"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Remaining  int       `json:"remaining_actions"`
	Stopped    bool      `json:"stopped"`
	redactions []Rect
}

type Action struct {
	SessionID string     `json:"session_id"`
	Kind      ActionKind `json:"kind"`
	Target    string     `json:"target"`
	X         int        `json:"x,omitempty"`
	Y         int        `json:"y,omitempty"`
	Text      string     `json:"text,omitempty"`
	Key       string     `json:"key,omitempty"`
	DeltaX    int        `json:"delta_x,omitempty"`
	DeltaY    int        `json:"delta_y,omitempty"`
}

type DriverResult struct {
	Message   string
	Capture   []byte
	MediaType string
}

type Driver interface {
	Available() bool
	Execute(context.Context, Action) (DriverResult, error)
}

type Result struct {
	Message  string              `json:"message,omitempty"`
	Artifact *artifacts.Metadata `json:"artifact,omitempty"`
}

type AuditEvent struct {
	SessionID string         `json:"session_id,omitempty"`
	Actor     string         `json:"actor,omitempty"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data,omitempty"`
	Time      time.Time      `json:"time"`
}

type Controller struct {
	mu        sync.Mutex
	driver    Driver
	broker    *capabilities.Broker
	artifacts *artifacts.Store
	sessions  map[string]*Session
	active    map[string]context.CancelFunc
	emergency bool
	audit     func(AuditEvent)
}

// Deprecated: this legacy scaffold uses session-wide approval and is not wired
// into the service. Native drivers must use the durable protocol-v2 supervisor,
// never this controller. Retained only for legacy isolation/redaction fixtures.
func NewController(driver Driver, broker *capabilities.Broker, store *artifacts.Store) *Controller {
	controller := &Controller{driver: driver, broker: broker, artifacts: store, sessions: make(map[string]*Session), active: make(map[string]context.CancelFunc)}
	for _, descriptor := range []capabilities.Descriptor{
		{Name: "computer.session", Namespace: "computer", Source: "local", Kind: capabilities.Computer, Risk: capabilities.RiskHigh, Description: "Start a scoped computer-use session"},
		{Name: "computer.action", Namespace: "computer", Source: "local", Kind: capabilities.Computer, Risk: capabilities.RiskHigh, Description: "Perform an action in an approved computer-use scope"},
		{Name: "computer.reset", Namespace: "computer", Source: "local", Kind: capabilities.Computer, Risk: capabilities.RiskHigh, Description: "Reset the computer-use emergency stop"},
	} {
		if broker != nil {
			_ = broker.Register(descriptor)
		}
	}
	return controller
}

func (c *Controller) SetAudit(callback func(AuditEvent)) { c.audit = callback }

func (c *Controller) Begin(ctx context.Context, actor string, scope Scope, approved bool) (*Session, error) {
	if scope.Target == "" {
		return nil, fmt.Errorf("computer-use target is required")
	}
	if err := c.authorize(ctx, "computer.session", actor, approved, map[string]any{"target": scope.Target}); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.emergency {
		return nil, fmt.Errorf("computer-use emergency stop is active")
	}
	if c.driver == nil || !c.driver.Available() {
		return nil, fmt.Errorf("no computer-use driver is available")
	}
	duration := scope.ExpiresIn
	if scope.ExpiresSeconds > 0 {
		duration = time.Duration(scope.ExpiresSeconds) * time.Second
	}
	if duration <= 0 || duration > 30*time.Minute {
		duration = 10 * time.Minute
	}
	if scope.MaxActions <= 0 || scope.MaxActions > 1000 {
		scope.MaxActions = 100
	}
	now := time.Now().UTC()
	session := &Session{ID: randomID(), Actor: actor, Target: scope.Target, CreatedAt: now, ExpiresAt: now.Add(duration), Remaining: scope.MaxActions, redactions: append([]Rect(nil), scope.Redactions...)}
	c.sessions[session.ID] = session
	c.emit(AuditEvent{SessionID: session.ID, Actor: actor, Type: "computer.session.started", Data: map[string]any{"target": scope.Target}, Time: now})
	copy := *session
	return &copy, nil
}

func (c *Controller) Execute(ctx context.Context, actor string, action Action) (Result, error) {
	c.mu.Lock()
	session := c.sessions[action.SessionID]
	if session == nil || session.Stopped || c.emergency || time.Now().After(session.ExpiresAt) || session.Remaining <= 0 {
		c.mu.Unlock()
		return Result{}, fmt.Errorf("computer-use session is unavailable, expired, or stopped")
	}
	if session.Actor != actor || action.Target != session.Target {
		c.mu.Unlock()
		return Result{}, fmt.Errorf("computer-use action is outside the approved scope")
	}
	if !validAction(action.Kind) {
		c.mu.Unlock()
		return Result{}, fmt.Errorf("unsupported computer-use action: %s", action.Kind)
	}
	session.Remaining--
	redactions := append([]Rect(nil), session.redactions...)
	actionID := randomID()
	actionContext, cancelAction := context.WithCancel(ctx)
	c.active[actionID] = cancelAction
	c.mu.Unlock()
	defer func() {
		cancelAction()
		c.mu.Lock()
		delete(c.active, actionID)
		c.mu.Unlock()
	}()

	if err := c.authorize(ctx, "computer.action", actor, true, map[string]any{"session_id": action.SessionID, "kind": action.Kind, "target": action.Target}); err != nil {
		return Result{}, err
	}
	result, err := c.driver.Execute(actionContext, action)
	data := map[string]any{"kind": action.Kind, "target": action.Target, "success": err == nil}
	if action.Kind == Type {
		data["text_length"] = len(action.Text)
	}
	if err != nil {
		data["error"] = err.Error()
		c.emit(AuditEvent{SessionID: action.SessionID, Actor: actor, Type: "computer.action.completed", Data: data, Time: time.Now().UTC()})
		return Result{}, err
	}
	response := Result{Message: result.Message}
	if len(result.Capture) > 0 {
		capture, redactErr := redactCapture(result.Capture, result.MediaType, redactions)
		if redactErr != nil {
			return Result{}, redactErr
		}
		if c.artifacts != nil {
			metadata, storeErr := c.artifacts.Put(ctx, bytes.NewReader(capture), artifacts.Metadata{
				Name: action.SessionID + "-capture.png", MediaType: "image/png",
				Labels: map[string]string{"session_id": action.SessionID, "kind": "computer-capture", "redacted": "true"},
			})
			if storeErr != nil {
				return Result{}, storeErr
			}
			response.Artifact = &metadata
			data["artifact_digest"] = metadata.Digest
		}
	}
	c.emit(AuditEvent{SessionID: action.SessionID, Actor: actor, Type: "computer.action.completed", Data: data, Time: time.Now().UTC()})
	return response, nil
}

func (c *Controller) EmergencyStop(actor string) {
	c.mu.Lock()
	c.emergency = true
	for _, session := range c.sessions {
		session.Stopped = true
	}
	for _, cancel := range c.active {
		cancel()
	}
	c.mu.Unlock()
	c.emit(AuditEvent{Actor: actor, Type: "computer.emergency_stop", Time: time.Now().UTC()})
}

func (c *Controller) Reset(ctx context.Context, actor string, approved bool) error {
	if err := c.authorize(ctx, "computer.reset", actor, approved, nil); err != nil {
		return err
	}
	c.mu.Lock()
	c.emergency = false
	c.mu.Unlock()
	c.emit(AuditEvent{Actor: actor, Type: "computer.emergency_reset", Time: time.Now().UTC()})
	return nil
}

func (c *Controller) Status() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	active := 0
	for _, session := range c.sessions {
		if !session.Stopped && time.Now().Before(session.ExpiresAt) && session.Remaining > 0 {
			active++
		}
	}
	return map[string]any{"available": c.driver != nil && c.driver.Available(), "emergency_stop": c.emergency, "active_sessions": active}
}

func (c *Controller) authorize(ctx context.Context, name, actor string, approved bool, arguments map[string]any) error {
	if c.broker == nil {
		return fmt.Errorf("capability broker is unavailable")
	}
	descriptor, ok := c.broker.Resolve(name)
	if !ok {
		return fmt.Errorf("computer capability is not registered")
	}
	_, err := c.broker.Authorize(ctx, capabilities.Request{Actor: actor, Capability: descriptor, Arguments: arguments, Approved: approved})
	return err
}

func (c *Controller) emit(event AuditEvent) {
	if c.audit != nil {
		c.audit(event)
	}
}

func validAction(kind ActionKind) bool {
	return kind == Capture || kind == Click || kind == Type || kind == Key || kind == Scroll
}

func redactCapture(content []byte, mediaType string, redactions []Rect) ([]byte, error) {
	var source image.Image
	var err error
	if mediaType == "image/jpeg" {
		source, err = jpeg.Decode(bytes.NewReader(content))
	} else {
		source, err = png.Decode(bytes.NewReader(content))
	}
	if err != nil {
		return nil, fmt.Errorf("decode computer capture: %w", err)
	}
	canvas := image.NewRGBA(source.Bounds())
	draw.Draw(canvas, canvas.Bounds(), source, source.Bounds().Min, draw.Src)
	for _, redaction := range redactions {
		rectangle := image.Rect(redaction.X, redaction.Y, redaction.X+redaction.Width, redaction.Y+redaction.Height).Intersect(canvas.Bounds())
		draw.Draw(canvas, rectangle, &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic("OS random source unavailable")
	}
	return hex.EncodeToString(value)
}

// UnsupportedDriver makes unavailable deployments explicit and safe.
type UnsupportedDriver struct{}

func (UnsupportedDriver) Available() bool { return false }
func (UnsupportedDriver) Execute(context.Context, Action) (DriverResult, error) {
	return DriverResult{}, fmt.Errorf("computer-use driver unavailable")
}
