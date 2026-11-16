package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func validateConversationInput(c api.ConversationInputCommand) error {
	if len(c.Payload.KnowledgeSelections) > 16 {
		return ErrControlCommand
	}
	selected := map[string]bool{}
	for _, source := range c.Payload.KnowledgeSelections {
		if !validProfileText(source.DocumentID, 256, true) || !validProfileText(source.Revision, 128, true) || source.Page < 0 || source.Page > 100000 || len(source.BlockIDs) > 32 || (source.Page > 0 && len(source.BlockIDs) > 0) || (source.Page == 0 && len(source.BlockIDs) == 0) || selected[source.DocumentID] {
			return ErrControlCommand
		}
		selected[source.DocumentID] = true
		seen := map[string]bool{}
		for _, id := range source.BlockIDs {
			if !validProfileText(id, 128, true) || seen[id] {
				return ErrControlCommand
			}
			seen[id] = true
		}
	}
	if len(c.Payload.Attachments) > 8 || len(c.Payload.KnowledgeDocuments) > 16 || api.ValidateSessionImages(c.Payload.Images) != nil {
		return ErrControlCommand
	}
	if len(c.Payload.Attachments) > 0 && !validProfileText(c.Payload.SourceSession, 256, true) {
		return ErrControlCommand
	}
	seen := map[string]bool{}
	for _, source := range c.Payload.Attachments {
		if !validTaskID(source.ID) || len(source.Revision) != 64 || source.Page < 0 || source.Page > 100000 || seen[source.ID] {
			return ErrControlCommand
		}
		if _, err := hex.DecodeString(source.Revision); err != nil {
			return ErrControlCommand
		}
		seen[source.ID] = true
	}
	for _, id := range c.Payload.KnowledgeDocuments {
		if !validProfileText(id, 256, true) {
			return ErrControlCommand
		}
	}
	if (c.Payload.Intent == "answer" || c.Payload.Intent == "correction") && (len(c.Payload.Attachments)+len(c.Payload.Images)+len(c.Payload.KnowledgeDocuments)+len(c.Payload.KnowledgeSelections) > 0 || c.Payload.UseKnowledge) {
		return ErrControlCommand
	}
	if !validProfileText(c.RequestID, 128, true) || strings.ContainsAny(c.RequestID, "\r\n") || c.Kind != "submit_input" || !validTaskID(c.TargetID) || c.ExpectedRevision < 1 || !validProfileText(c.Payload.Content, 32000, true) || strings.TrimSpace(c.Payload.Content) == "" || !validProfileText(c.Payload.Model, 256, false) {
		return ErrControlCommand
	}
	switch c.Payload.Intent {
	case "", "message", "correction", "new_work", "answer":
	default:
		return ErrControlCommand
	}
	if c.Payload.Intent == "answer" {
		if !validTaskID(c.Payload.QuestionID) || !validProfileText(c.Payload.Content, 16000, true) {
			return ErrControlCommand
		}
	} else if c.Payload.QuestionID != "" {
		return ErrControlCommand
	}
	switch c.Payload.Origin {
	case "", "typed", "dictation", "steering":
	default:
		return ErrControlCommand
	}
	if focus := c.Payload.FocusedWorkID; focus != nil && *focus != "" && (!validTaskID(*focus) || c.Payload.Intent == "new_work") {
		return ErrControlCommand
	}
	return nil
}

func conversationInputDigest(c api.ConversationInputCommand) string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(append([]byte("conversation-input-v1\x00"), data...))
	return hex.EncodeToString(hash[:])
}

// AcceptConversationInput commits the inbox, user history and graph fences
// together. It never waits for a model, calls a tool or replenishes work budget.
// The normal foreground turn incorporates the input separately after commit.
func (r *Runner) AcceptConversationInput(scope ControlScope, c api.ConversationInputCommand) (*ConversationCommandReceipt, error) {
	if err := validateConversationInput(c); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return nil, ErrRunConflict
	}
	db, err := r.manager.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	var publish []*Task
	receipt, err := withControlCommandDatabase(db, scope, c.RequestID, conversationInputDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		record, err := readControlRecord(tx, scope, c.TargetID, "conversation")
		if err != nil {
			return nil, err
		}
		if record.Revision != c.ExpectedRevision {
			return nil, ErrRunConflict
		}
		var conversation Conversation
		if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
			return nil, err
		}
		var pending int
		if err = tx.QueryRow(`SELECT count(*) FROM agent_conversation_inbox WHERE workspace=? AND actor=? AND conversation_id=? AND state IN ('accepted','processing')`, scope.Workspace, scope.Actor, conversation.ID).Scan(&pending); err != nil {
			return nil, err
		}
		if pending >= 32 {
			return nil, ErrRunConflict
		}
		focus := conversation.FocusedWorkID
		if c.Payload.FocusedWorkID != nil {
			focus = *c.Payload.FocusedWorkID
		}
		if c.Payload.Intent == "new_work" {
			focus = ""
		}
		if focus == "" && (c.Payload.Intent == "correction" || c.Payload.Intent == "answer") {
			return nil, ErrControlCommand
		}
		conversation.Revision++
		entry := &api.ConversationInboxEntry{ID: "input-" + runID(), ConversationID: conversation.ID, ConversationRevision: conversation.Revision, RequestID: c.RequestID, WorkID: focus, Input: c.Payload, State: "accepted", AcceptedAt: time.Now().UTC()}
		if focus != "" {
			work, err := linkedInputWork(tx, scope, conversation.ID, focus)
			if err != nil {
				return nil, err
			}
			root := r.manager.tasks[work.RunID]
			if root == nil || root.Actor != scope.Actor || root.DeletedAt != nil || root.WorkID != work.ID || root.Origin == nil || root.Origin.Workspace != scope.Workspace {
				return nil, ErrControlNotFound
			}
			entry.InstructionVersion = root.InstructionVersion
			if c.Payload.Intent == "answer" {
				publish, err = r.acceptConversationQuestionAnswer(tx, scope, root, entry)
				if err != nil {
					return nil, err
				}
			} else if !terminalControlTask(root) {
				ids := []string{root.ID}
				for _, link := range root.ChildHistory {
					ids = append(ids, link.ID)
				}
				for _, id := range ids {
					current := r.manager.tasks[id]
					if current == nil || current.Actor != scope.Actor || current.DeletedAt != nil || id != root.ID && current.ParentID != root.ID {
						return nil, ErrRunStorage
					}
					if terminalControlTask(current) {
						continue
					}
					next, err := copyTask(current)
					if err != nil {
						return nil, err
					}
					if err = holdForConversationInput(next, entry.ID); err != nil {
						return nil, err
					}
					if err = putTaskTransaction(tx, next); err != nil {
						return nil, err
					}
					publish = append(publish, next)
				}
			}
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		insert, err := tx.Exec(`INSERT INTO agent_conversation_inbox(workspace,actor,conversation_id,input_id,state,payload) VALUES(?,?,?,?,?,?)`, scope.Workspace, scope.Actor, conversation.ID, entry.ID, entry.State, data)
		if err != nil {
			return nil, err
		}
		if entry.Sequence, err = insert.LastInsertId(); err != nil {
			return nil, err
		}
		data, err = json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(`UPDATE agent_conversation_inbox SET payload=? WHERE sequence=?`, data, entry.Sequence); err != nil {
			return nil, err
		}
		history, err := json.Marshal(map[string]any{"id": entry.ID, "role": "user", "content": entry.Input.Content, "work_id": focus, "instruction_version": entry.InstructionVersion, "input_state": entry.State})
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(`INSERT INTO agent_conversation_messages(workspace,actor,conversation_id,message_id,payload) VALUES(?,?,?,?,?)`, scope.Workspace, scope.Actor, conversation.ID, entry.ID, history); err != nil {
			return nil, err
		}
		if err = putControlRecord(tx, scope, conversation.ID, "conversation", conversation.Revision, &conversation); err != nil {
			return nil, err
		}
		cursor, err := appendControlEvent(tx, scope, conversation.ID, conversation.Revision, "input_accepted", entry, "")
		if err != nil {
			return nil, err
		}
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: conversation.ID, Revision: conversation.Revision, EventCursor: cursor, Conversation: &conversation, Input: entry}, nil
	})
	if err == nil {
		for _, task := range publish {
			r.manager.tasks[task.ID] = task
		}
	}
	return receipt, err
}

func linkedInputWork(q controlReader, scope ControlScope, conversation, workID string) (*WorkItem, error) {
	var data []byte
	err := q.QueryRow(`SELECT w.snapshot FROM agent_control_links l JOIN agent_control_records w ON w.workspace=l.workspace AND w.actor=l.actor AND w.id=l.work_id AND w.kind='work' WHERE l.workspace=? AND l.actor=? AND l.conversation_id=? AND l.work_id=?`, scope.Workspace, scope.Actor, conversation, workID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	var work WorkItem
	err = json.Unmarshal(data, &work)
	return &work, err
}
