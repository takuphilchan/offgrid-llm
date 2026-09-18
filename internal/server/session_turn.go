package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (h *SessionHandlers) generateDurable(w http.ResponseWriter, r *http.Request, name, id, prompt, model string, knowledge bool, profile string, tokens int) {
	if !isSafeModelID(id) || len(id) > 128 || h.streamer == nil {
		writeError(w, "A valid request_id and streaming runtime are required", 400)
		return
	}
	scoped := h.scoped(r)
	session, err := scoped.Load(name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if model == "" {
		model = session.ModelID
	}
	if model == "" {
		writeError(w, "Model is required", 400)
		return
	}
	turn := &sessions.Turn{ID: id, Prompt: prompt, Model: model, Knowledge: knowledge, Profile: profile, MaxTokens: tokens, Status: "pending", Phase: "queued"}
	encoded, _ := json.Marshal(turn)
	hash := sha256.Sum256(encoded)
	turn.RequestHash = hex.EncodeToString(hash[:])
	h.turnMu.Lock()
	if h.turnClosed {
		h.turnMu.Unlock()
		writeError(w, "Service is stopping", 503)
		return
	}
	if _, live := h.turnCancels[name]; live && (session.Turn == nil || session.Turn.ID != id) {
		h.turnMu.Unlock()
		writeSessionError(w, sessions.ErrTurnConflict)
		return
	}
	turn, fresh, err := scoped.BeginTurn(name, turn)
	if err != nil {
		h.turnMu.Unlock()
		writeSessionError(w, err)
		return
	}
	if fresh {
		// Preserve trusted actor context but do not attach execution to a browser.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Minute)
		base := h.runtimeContext
		if base == nil {
			base = context.Background()
		}
		stop := context.AfterFunc(base, cancel)
		if h.turnCancels == nil {
			h.turnCancels = map[string]context.CancelFunc{}
		}
		h.turnCancels[name] = cancel
		h.turnWorkers.Add(1)
		go func() {
			defer h.turnWorkers.Done()
			defer cancel()
			defer stop()
			defer func() { h.turnMu.Lock(); delete(h.turnCancels, name); h.turnMu.Unlock() }()
			h.executeTurn(ctx, scoped, name, turn)
		}()
	}
	h.turnMu.Unlock()
	h.followTurn(w, r, name, id)
}

func (h *SessionHandlers) executeTurn(ctx context.Context, scoped *sessions.ScopedManager, name string, turn *sessions.Turn) {
	// BeginTurn persisted admission before this worker was started.
	unlock, err := h.lockSessionContext(ctx, name)
	if err == nil {
		defer unlock()
		session, loadErr := scoped.Load(name)
		err = loadErr
		if err == nil {
			messages := make([]api.ChatMessage, 0, len(session.Messages)+1)
			for _, msg := range session.Messages {
				messages = append(messages, api.ChatMessage{Role: msg.Role, Content: msg.Content})
			}
			messages = append(messages, api.ChatMessage{Role: "user", Content: turn.Prompt})
			lastSave := time.Time{}
			emit := func(event ChatStreamEvent) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				turn.Status = "running"
				if event.Type == "phase" {
					turn.Phase = event.Phase
				}
				if event.Type == "delta" {
					turn.Output += event.Delta
				}
				if len(turn.Output) > 8<<20 {
					return fmt.Errorf("response exceeded safe size limit")
				}
				if event.Type == "phase" || time.Since(lastSave) >= 100*time.Millisecond {
					if err := scoped.SaveTurn(name, turn); err != nil {
						return err
					}
					lastSave = time.Now()
				}
				return nil
			}
			var result ChatStreamResult
			result, err = h.streamer(ctx, &api.ChatCompletionRequest{Model: turn.Model, Messages: messages, UseKnowledgeBase: &turn.Knowledge, MaxTokens: &turn.MaxTokens, Stream: true}, turn.Profile, emit)
			if err == nil {
				err = ctx.Err()
			}
			if err == nil && result.Answer == "" {
				err = fmt.Errorf("empty model response")
			}
			if err == nil {
				turn.Status = "completed"
				turn.Output = result.Answer
				turn.FinishReason = result.FinishReason
				turn.Metrics, _ = json.Marshal(result.Metrics)
			}
		}
	}
	if err != nil {
		turn.Status = "failed"
		turn.Error = "Response could not finish. Your draft and partial output are retained; review before retrying."
		if ctx.Err() != nil {
			turn.Status = "cancelled"
			turn.Error = "Generation stopped. Partial output is not completed conversational context."
		}
		if h.runtimeContext != nil && h.runtimeContext.Err() != nil {
			turn.Status = "interrupted"
			turn.Error = "Service stopped before completion. Review before retrying."
		}
	}
	if saveErr := scoped.SaveTurn(name, turn); saveErr != nil {
		// Never publish success if the final durable write failed.
		// A subsequent read with no live worker reports the persisted turn interrupted.
		return
	}
}

func (h *SessionHandlers) currentTurn(r *http.Request, name string) (*sessions.Session, error) {
	h.turnMu.Lock()
	session, err := h.scoped(r).Load(name)
	if err != nil {
		h.turnMu.Unlock()
		return nil, err
	}
	_, live := h.turnCancels[name]
	h.turnMu.Unlock()
	if session.Turn.Active() && !live {
		session.Turn.Status = "interrupted"
		session.Turn.Error = "The worker stopped before saving completion. Review retained output before retrying."
		if err := h.scoped(r).SaveTurn(name, session.Turn); err != nil {
			return nil, err
		}
	}
	return session, nil
}

func (h *SessionHandlers) handleTurn(w http.ResponseWriter, r *http.Request, name, action string) {
	session, err := h.currentTurn(r, name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if action == "turn" && r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"turn": session.Turn})
		return
	}
	if action == "turn/events" && r.Method == "GET" {
		h.followTurn(w, r, name, r.URL.Query().Get("id"))
		return
	}
	if action == "turn/cancel" && r.Method == "POST" {
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || session.Turn == nil || req.ID != session.Turn.ID {
			writeError(w, "Turn changed; refresh before stopping it", 409)
			return
		}
		// Re-read under the same lock used by admission. Without this check a
		// delayed cancel for turn A could stop a newly admitted turn B.
		h.turnMu.Lock()
		current, loadErr := h.scoped(r).Load(name)
		cancel := h.turnCancels[name]
		if loadErr != nil || current.Turn == nil || current.Turn.ID != req.ID || !current.Turn.Active() || cancel == nil {
			h.turnMu.Unlock()
			writeError(w, "Turn is no longer running; refresh before stopping it", 409)
			return
		}
		cancel()
		h.turnMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}
	writeError(w, "Method not allowed", 405)
}

// Each attachment reconstructs a complete snapshot; disconnect only detaches.
// Slow consumers cannot block the worker or prevent its durable commit.
func (h *SessionHandlers) followTurn(w http.ResponseWriter, r *http.Request, name, id string) {
	session, err := h.currentTurn(r, name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if session.Turn == nil || session.Turn.ID != id {
		writeError(w, "Turn changed; reload the conversation", 409)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "Streaming unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	offset := 0
	phase := ""
	lastHeartbeat := time.Now()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	write := func(event ChatStreamEvent) error {
		data, _ := json.Marshal(event)
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return err
	}
	defer http.NewResponseController(w).SetWriteDeadline(time.Time{})
	for {
		turn := session.Turn
		if turn == nil || turn.ID != id {
			return
		}
		if turn.Phase != phase {
			phase = turn.Phase
			if write(ChatStreamEvent{Type: "phase", Phase: phase}) != nil {
				return
			}
		}
		if len(turn.Output) > offset {
			if write(ChatStreamEvent{Type: "delta", Delta: turn.Output[offset:]}) != nil {
				return
			}
			offset = len(turn.Output)
		}
		if !turn.Active() {
			if turn.Status == "completed" {
				var metrics ChatTimings
				_ = json.Unmarshal(turn.Metrics, &metrics)
				_ = write(ChatStreamEvent{Type: "done", Session: session, Message: &sessions.Message{Role: "assistant", Content: turn.Output, Timestamp: session.UpdatedAt}, Metrics: &metrics, FinishReason: turn.FinishReason})
			} else {
				_ = write(ChatStreamEvent{Type: "error", Error: turn.Error})
			}
			return
		}
		if time.Since(lastHeartbeat) > 8*time.Second {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
			lastHeartbeat = time.Now()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
		session, err = h.currentTurn(r, name)
		if err != nil {
			_ = write(ChatStreamEvent{Type: "error", Error: "Conversation is no longer accessible"})
			return
		}
	}
}

func (h *SessionHandlers) closeTurns() {
	h.turnMu.Lock()
	h.turnClosed = true
	for _, cancel := range h.turnCancels {
		cancel()
	}
	h.turnMu.Unlock()
	h.turnWorkers.Wait()
}
