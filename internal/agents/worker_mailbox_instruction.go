package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Shared by direct worker commands and canonical conversation answers. The
// caller validates the assignment and commits this alongside its task changes.
func appendWorkerMailboxInstruction(tx *sql.Tx, scope ControlScope, next, parent *Task, c api.WorkerCommand) (*api.WorkerMessage, error) {
	now := time.Now().UTC()
	message := &api.WorkerMessage{ID: c.RequestID, WorkerID: c.TargetID, RunID: next.ID, Content: c.Payload.Content, State: "accepted", InstructionVersion: parent.InstructionVersion, AcceptedAt: now, QuestionID: c.Payload.QuestionID}
	data, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_worker_messages(workspace,actor,worker_id,message_id,payload) VALUES(?,?,?,?,?)`, scope.Workspace, scope.Actor, c.TargetID, c.RequestID, data); err != nil {
		return nil, err
	}
	next.ControlEpoch++
	if next.WorkerMessageRequests == nil {
		next.WorkerMessageRequests = map[string]bool{}
	}
	next.WorkerMessageRequests[c.RequestID] = true
	digest := sha256.Sum256([]byte(c.Payload.Content))
	next.PendingInstructions = append(next.PendingInstructions, Instruction{RequestID: c.RequestID, Digest: hex.EncodeToString(digest[:]), Text: c.Payload.Content, At: now})
	return message, nil
}
