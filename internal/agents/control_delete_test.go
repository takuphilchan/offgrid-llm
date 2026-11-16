package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestImportedConversationDeletionIsolationPendingAndRetry(t *testing.T) {
	m, _, scope, _ := controlFixture(t)
	input := ConversationImport{LegacyName: "private-chat", Source: []byte("{\"messages\":[\"private text\"]}"), Messages: []json.RawMessage{json.RawMessage("{\"content\":\"private text\"}")}}
	manifest, err := m.ImportConversation(scope, input)
	if err != nil {
		t.Fatal(err)
	}
	id := manifest.ConversationID
	other := ControlScope{Workspace: scope.Workspace, Actor: "bob"}
	if err = m.DeleteImportedConversation(other, input.LegacyName, id, 1, true); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("cross actor delete", err)
	}
	if err = m.DeleteImportedConversation(scope, input.LegacyName, id, 2, true); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale delete", err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO agent_conversation_inbox(workspace,actor,conversation_id,input_id,state,payload) VALUES(?,?,?,?,?,?)", scope.Workspace, scope.Actor, id, "pending", "accepted", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteImportedConversation(scope, input.LegacyName, id, 1, true); !errors.Is(err, ErrConversationInputPending) {
		t.Fatal("discarded pending input", err)
	}
	if _, err = db.Exec("UPDATE agent_conversation_inbox SET state='responded',payload=? WHERE input_id='pending'", []byte(`{"decision":{"kind":"answered"}}`)); err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteImportedConversation(scope, input.LegacyName, id, 1, true); !errors.Is(err, ErrConversationInputPending) {
		t.Fatal("discarded answer awaiting hold release", err)
	}
	if _, err = db.Exec("DELETE FROM agent_conversation_inbox WHERE input_id='pending'"); err != nil {
		t.Fatal(err)
	}
	run := "run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	data, _ := json.Marshal(&Task{ID: run, Actor: scope.Actor, Status: TaskRunning})
	if _, err = db.Exec("INSERT INTO agent_tasks(id,actor,snapshot) VALUES(?,?,?)", run, scope.Actor, data); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = putControlRecord(tx, scope, "work-retained", "work", 1, &WorkItem{ID: "work-retained", RunID: run}); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO agent_control_links(workspace,actor,conversation_id,work_id) VALUES(?,?,?,?)", scope.Workspace, scope.Actor, id, "work-retained"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteImportedConversation(scope, input.LegacyName, id, 1, false); !errors.Is(err, ErrConversationHasWork) {
		t.Fatal("silently retained active work", err)
	}
	for i := 0; i < 2; i++ {
		if err = m.DeleteImportedConversation(scope, input.LegacyName, id, 1, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = m.ImportConversation(scope, input); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("reimport resurrected deleted chat", err)
	}
	var kept int
	if err = db.QueryRow("SELECT count(*) FROM agent_tasks WHERE id=?", run).Scan(&kept); err != nil || kept != 1 {
		t.Fatal("deleted retained work", kept, err)
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM agent_conversation_messages WHERE conversation_id=?", id).Scan(&count); err != nil || count != 0 {
		t.Fatal("retained conversation text", count, err)
	}
	var body []byte
	if err = db.QueryRow("SELECT source FROM agent_conversation_imports WHERE conversation_id=?", id).Scan(&body); err != nil || string(body) != "{}" {
		t.Fatal("retained import body", err)
	}
	var failures int
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		failures++
	}
	rows.Close()
	if failures != 0 {
		t.Fatal("foreign key damage", failures)
	}
}

func TestConversationDeleteSerializesWithRename(t *testing.T) {
	m, _, scope, _ := controlFixture(t)
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("race-%d", i)
		manifest, err := m.ImportConversation(scope, ConversationImport{LegacyName: name, Source: []byte(`{"messages":[]}`)})
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		removed, renamed := make(chan error, 1), make(chan error, 1)
		go func() {
			<-start
			removed <- m.DeleteImportedConversation(scope, name, manifest.ConversationID, 1, true)
		}()
		go func() {
			<-start
			_, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "rename-" + name, TargetID: manifest.ConversationID, ExpectedRevision: 1, Kind: "rename_conversation", Payload: ConversationCommandPayload{Title: "Changed"}})
			renamed <- err
		}()
		close(start)
		deleteErr, renameErr := <-removed, <-renamed
		if deleteErr == nil {
			if !errors.Is(renameErr, ErrControlNotFound) {
				t.Fatal("rename survived deletion", renameErr)
			}
		} else {
			if !errors.Is(deleteErr, ErrRunConflict) || renameErr != nil {
				t.Fatal("unexpected race outcome", deleteErr, renameErr)
			}
			if err = m.DeleteImportedConversation(scope, name, manifest.ConversationID, 2, true); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = m.Conversation(scope, manifest.ConversationID); !errors.Is(err, ErrControlNotFound) {
			t.Fatal("deleted conversation revived", err)
		}
	}
}

func TestConversationDeleteTransactionRollsBackOnPurgeFailure(t *testing.T) {
	m, _, scope, _ := controlFixture(t)
	input := ConversationImport{LegacyName: "rollback", Source: []byte(`{"messages":[]}`)}
	manifest, err := m.ImportConversation(scope, input)
	if err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_delete_purge BEFORE UPDATE ON agent_conversation_imports BEGIN SELECT RAISE(ABORT,'purge fault'); END`); err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteImportedConversation(scope, input.LegacyName, manifest.ConversationID, 1, true); err == nil {
		t.Fatal("purge failure ignored")
	}
	if _, err = m.Conversation(scope, manifest.ConversationID); err != nil {
		t.Fatal("rollback hid conversation", err)
	}
	if body, err := m.ImportedConversationSource(scope, manifest.ConversationID); err != nil || string(body) != string(input.Source) {
		t.Fatal("rollback lost original", err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_delete_purge`); err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteImportedConversation(scope, input.LegacyName, manifest.ConversationID, 1, true); err != nil {
		t.Fatal(err)
	}
}
