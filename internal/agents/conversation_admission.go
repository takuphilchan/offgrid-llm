package agents

import "strings"

// ConversationOrigin is supplied by the service coordinator, never model
// output. A held job is persisted before the separate session-file association.
type ConversationOrigin struct {
	Workspace    string `json:"workspace"`
	Conversation string `json:"conversation"`
	TurnID       string `json:"turn_id"`
}

func (o ConversationOrigin) valid() bool {
	for _, part := range []string{o.Workspace, o.Conversation, o.TurnID} {
		if part == "" || len(part) > 256 || strings.ContainsAny(part, "\x00\r\n") {
			return false
		}
	}
	return true
}

func sameConversationOrigin(a, b *ConversationOrigin) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// CreateHeldRequest has no executable admission path until the coordinator
// durably links this exact origin and calls ReleaseConversationAdmission.
func (r *Runner) CreateHeldRequest(prompt, model, actor string, config AgentConfig, requestID, digest string, origin ConversationOrigin) (*Task, bool, error) {
	if !origin.valid() || requestID == "" || digest == "" {
		return nil, false, ErrRunConflict
	}
	return r.createRequest(prompt, model, actor, config, requestID, digest, &origin)
}

// ReleaseConversationAdmission records linkage, but never starts or resumes
// execution. It is deliberately not an HTTP action: only the coordinator may
// call it after verifying the durable session association. Repetition observes
// the same job, including interrupted, stopped, completed and uncertain jobs.
func (r *Runner) ReleaseConversationAdmission(id, actor, digest string, origin ConversationOrigin) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing || !origin.valid() {
		return nil, ErrRunConflict
	}
	current, exists := r.manager.GetTask(id)
	if !exists || current.Actor != actor {
		return nil, ErrTaskNotFound
	}
	if current.RequestDigest != digest || !sameConversationOrigin(current.Origin, &origin) {
		return nil, ErrRunConflict
	}
	if !current.AdmissionHeld {
		return current, nil
	}
	return r.manager.updateTask(id, func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		if task.RequestDigest != digest || !sameConversationOrigin(task.Origin, &origin) {
			return ErrRunConflict
		}
		task.AdmissionHeld = false
		return nil
	})
}

func (r *Runner) WorkerActive(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, active := r.active[id]
	return active
}
