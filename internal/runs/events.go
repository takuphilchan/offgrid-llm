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
	RunStateChanged   EventType = "run.state_changed"
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
	identities  map[string]bool
	subscribers map[uint64]chan Event
	nextSubID   uint64
	storageErr  error
}

func NewLog(path string) (*Log, error) {
	if path == "" {
		return nil, fmt.Errorf("event log path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	log := &Log{path: path, sequences: make(map[string]uint64), identities: make(map[string]bool), subscribers: make(map[uint64]chan Event)}
	events, err := log.Replay("", 0)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, event := range events {
		log.identities[event.ID] = true
		if event.Sequence > log.sequences[event.RunID] {
			log.sequences[event.RunID] = event.Sequence
		}
	}
	return log, nil
}

func (l *Log) Publish(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.RunID == "" || event.Type == "" {
		return fmt.Errorf("run ID and event type are required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.storageErr != nil {
		return fmt.Errorf("event log requires recovery: %w", l.storageErr)
	}
	if event.ID == "" {
		event.ID = newID()
	}
	if l.identities[event.ID] {
		return fmt.Errorf("duplicate event ID")
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	event.Sequence = l.sequences[event.RunID] + 1
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(encoded) >= 4*1024*1024-1 {
		return fmt.Errorf("event exceeds durable log record limit")
	}
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
		// A partial append or failed sync has an uncertain on-disk outcome.
		// Do not append another event over it or reuse the sequence in-process.
		l.storageErr = err
		return err
	}
	l.sequences[event.RunID] = event.Sequence
	l.identities[event.ID] = true
	for id, subscriber := range l.subscribers {
		select {
		case subscriber <- event:
		default:
			// Disconnect lagging projections; replay is authoritative. Never
			// make model/tool execution wait for a disconnected UI consumer.
			close(subscriber)
			delete(l.subscribers, id)
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
			if subscriber, ok := l.subscribers[id]; ok {
				close(subscriber)
				delete(l.subscribers, id)
			}
			l.mu.Unlock()
		})
	}
}

func (l *Log) Replay(runID string, after uint64) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.storageErr != nil {
		return nil, fmt.Errorf("event log requires recovery: %w", l.storageErr)
	}
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
	sequences := make(map[string]uint64)
	identities := make(map[string]bool)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode event log: %w", err)
		}
		if event.ID == "" || event.RunID == "" || event.Type == "" || event.Time.IsZero() || identities[event.ID] || event.Sequence != sequences[event.RunID]+1 {
			return nil, fmt.Errorf("invalid or non-contiguous event log; preserve the file and repair from a verified backup")
		}
		identities[event.ID] = true
		sequences[event.RunID] = event.Sequence
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
