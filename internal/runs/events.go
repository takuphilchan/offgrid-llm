// Package runs provides a durable, transport-neutral event model for agent
// runs. HTTP, SSE, WebSocket, and external agent adapters can project the same
// event stream without owning execution state.
package runs

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type EventType string

const (
	RunStarted        EventType = "run.started"
	ModelDelta        EventType = "model.delta"
	ToolRequested     EventType = "tool.requested"
	ApprovalRequired  EventType = "approval.required"
	ToolCompleted     EventType = "tool.completed"
	ArtifactCreated   EventType = "artifact.created"
	RunCompleted      EventType = "run.completed"
	RunFailed         EventType = "run.failed"
	ComputerSession   EventType = "computer.session"
	ComputerAction    EventType = "computer.action"
	ComputerEmergency EventType = "computer.emergency"
)

type Event struct {
	ID          string          `json:"id"`
	RunID       string          `json:"run_id"`
	ParentRunID string          `json:"parent_run_id,omitempty"`
	AgentID     string          `json:"agent_id,omitempty"`
	Sequence    uint64          `json:"sequence"`
	Type        EventType       `json:"type"`
	Time        time.Time       `json:"time"`
	Data        json.RawMessage `json:"data,omitempty"`
}

func NewEvent(runID string, eventType EventType, data any) (Event, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return Event{}, err
	}
	return Event{ID: newID(), RunID: runID, Type: eventType, Time: time.Now().UTC(), Data: payload}, nil
}

type Log struct {
	mu          sync.Mutex
	path        string
	sequences   map[string]uint64
	subscribers map[uint64]chan Event
	nextSubID   uint64
}

func NewLog(path string) (*Log, error) {
	if path == "" {
		return nil, fmt.Errorf("event log path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	log := &Log{path: path, sequences: make(map[string]uint64), subscribers: make(map[uint64]chan Event)}
	events, err := log.Replay("", 0)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, event := range events {
		if event.Sequence > log.sequences[event.RunID] {
			log.sequences[event.RunID] = event.Sequence
		}
	}
	return log, nil
}

func (l *Log) Publish(ctx context.Context, event Event) error {
	if event.RunID == "" || event.Type == "" {
		return fmt.Errorf("run ID and event type are required")
	}
	l.mu.Lock()
	if event.ID == "" {
		event.ID = newID()
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	l.sequences[event.RunID]++
	event.Sequence = l.sequences[event.RunID]
	encoded, err := json.Marshal(event)
	if err == nil {
		var file *os.File
		file, err = os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_, err = file.Write(append(encoded, '\n'))
			if syncErr := file.Sync(); err == nil {
				err = syncErr
			}
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		l.sequences[event.RunID]--
		l.mu.Unlock()
		return err
	}
	subscribers := make([]chan Event, 0, len(l.subscribers))
	for _, subscriber := range l.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	l.mu.Unlock()
	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (l *Log) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	l.mu.Lock()
	id := l.nextSubID
	l.nextSubID++
	channel := make(chan Event, buffer)
	l.subscribers[id] = channel
	l.mu.Unlock()
	var once sync.Once
	return channel, func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.subscribers, id)
			l.mu.Unlock()
		})
	}
}

func (l *Log) Replay(runID string, after uint64) ([]Event, error) {
	file, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	defer file.Close()
	events := make([]Event, 0)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode event log: %w", err)
		}
		if (runID == "" || event.RunID == runID) && event.Sequence > after {
			events = append(events, event)
		}
	}
	return events, scanner.Err()
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
