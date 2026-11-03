package rag

import (
	"context"
	"sync"
	"time"
)

type retentionRunner struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	closed bool
	status RetentionStatus
}

type RetentionStatus struct {
	CheckedAt int64  `json:"checked_at"`
	Deleted   int    `json:"deleted"`
	ErrorCode string `json:"error_code,omitempty"`
}

func (e *Engine) RetentionStatus() RetentionStatus {
	e.retention.mu.Lock()
	defer e.retention.mu.Unlock()
	return e.retention.status
}

func (v *KnowledgeView) Retention(id string) (*DocumentRetention, error) {
	e := v.engine
	e.mu.RLock()
	defer e.mu.RUnlock()
	if err := e.documentAccess(v.principal, id, false); err != nil {
		return nil, err
	}
	s, ok := e.store.(*SQLiteStore)
	if !ok {
		return nil, ErrRetentionUnavailable
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return retentionPolicy(s.db, id)
}

func (v *KnowledgeView) SetRetention(id string, revision int, deleteAfter int64, hold bool) (*DocumentRetention, error) {
	e := v.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if !validKnowledgeActor(v.principal.actor) {
		return nil, ErrCollectionUnavailable
	}
	s, ok := e.store.(*SQLiteStore)
	if !ok || e.storageErr != nil {
		return nil, ErrRetentionUnavailable
	}
	return s.SetDocumentRetention(v.principal.actor, id, revision, deleteAfter, hold)
}

// ExpireDocuments is service-owned maintenance, never a model tool. Only
// explicitly saved expiry policies are eligible; migrated documents retain zero.
func (e *Engine) ExpireDocuments(now time.Time, limit int) (int, error) {
	return e.expireDocuments(context.Background(), now, limit)
}

func (e *Engine) expireDocuments(ctx context.Context, now time.Time, limit int) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s, ok := e.store.(*SQLiteStore)
	if !ok || e.storageErr != nil {
		return 0, ErrRetentionUnavailable
	}
	n, err := s.expireDocuments(now, limit)
	if err != nil {
		return n, err
	}
	cleanup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return n, e.cleanRebuildStaging(cleanup, s)
}

// StartRetention is called by application composition, not by test constructors.
// It owns one bounded sweep per minute and is joined before storage closes.
func (e *Engine) StartRetention() {
	r := &e.retention
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.cancel != nil || e.storageErr != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel, r.done = cancel, make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			// Failed deletion remains stored and is retried by a later sweep.
			n, err := e.expireDocuments(ctx, time.Now(), 100)
			status := RetentionStatus{CheckedAt: time.Now().Unix(), Deleted: n}
			if err != nil {
				status.ErrorCode = "document_retention_unavailable"
			}
			r.mu.Lock()
			r.status = status
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (e *Engine) stopRetention() {
	r := &e.retention
	r.mu.Lock()
	r.closed = true
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
