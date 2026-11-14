package agents

import (
	"encoding/json"
	"testing"
)

func TestImportCommandReceiptAndAuthoritySwitchRollbackTogether(t *testing.T) {
	m, _, scope, _ := controlFixture(t)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_import BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='import' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	command := ConversationCommand{RequestID: "import", Kind: "import_conversation", Payload: ConversationCommandPayload{LegacyName: "old-chat"}}
	input := ConversationImport{LegacyName: "old-chat", Source: []byte(`{"name":"old-chat","messages":[]}`), Messages: []json.RawMessage{json.RawMessage(`{"role":"user","content":"Keep this"}`)}}
	if _, err = m.ImportConversationCommand(scope, command, input); err == nil {
		t.Fatal("receipt failure ignored")
	}
	if manifest, err := m.ImportedConversation(scope, input.LegacyName); err != nil || manifest != nil {
		t.Fatal("uncommitted ownership switch", manifest, err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_conversation_messages`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial imported history", count, err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_import`); err != nil {
		t.Fatal(err)
	}
	accepted, err := m.ImportConversationCommand(scope, command, input)
	if err != nil || accepted.Import == nil {
		t.Fatal(accepted, err)
	}
	retry, err := m.ConversationCommandReceipt(scope, command)
	if err != nil || retry.TargetID != accepted.TargetID || retry.Import.SourceSHA256 != accepted.Import.SourceSHA256 {
		t.Fatal(retry, err)
	}
}
