package agents

import (
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationDeliveryCandidatesAreScopedHeadsAndPaginated(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	first := acceptTestConversationInput(t, r, scope, c, "first")
	acceptTestConversationInput(t, r, scope, c, "second")
	otherScope := ControlScope{Workspace: scope.Workspace, Actor: "bob"}
	other, err := m.ApplyConversationCommand(otherScope, ConversationCommand{RequestID: "other", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	otherInput := acceptTestConversationInput(t, r, otherScope, other.Conversation, "other-input")
	page, err := m.ConversationDeliveries(scope.Workspace, 0, 1)
	if err != nil || len(page.Items) != 1 || !page.HasMore || page.Items[0].Scope != scope || page.Items[0].Input.ID != first.ID {
		t.Fatal(page, err)
	}
	page, err = m.ConversationDeliveries(scope.Workspace, page.NextCursor, 1)
	if err != nil || len(page.Items) != 1 || page.HasMore || page.Items[0].Scope != otherScope || page.Items[0].Input.ID != otherInput.ID {
		t.Fatal(page, err)
	}
	page, err = m.ConversationDeliveries("other-workspace", 0, 100)
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	c, _ = m.Conversation(scope, c.ID)
	if _, err = r.AdmitConversationTurn(scope, c.ID, first.ID, c.Revision, "model", taskConfig()); err != nil {
		t.Fatal(err)
	}
	page, err = m.ConversationDeliveries(scope.Workspace, 0, 100)
	if err != nil || len(page.Items) != 1 || page.Items[0].Input.ID != otherInput.ID {
		t.Fatal("processing conversation contributed another head", page, err)
	}
}

func TestConversationDeliveryDeferralRollsBackAndCannotOverwriteAdmission(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "input")
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_deferral BEFORE INSERT ON agent_control_events WHEN NEW.kind='input_delivery_deferred' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	issue := api.ConversationDeliveryIssue{Code: "model_unavailable", Message: "Model unavailable", RetryAt: time.Now().Add(time.Minute).UTC()}
	if err = m.DeferConversationDelivery(scope, c.ID, input.ID, issue); err == nil {
		t.Fatal("fault ignored")
	}
	entry, err := m.ConversationInput(scope, c.ID, input.ID)
	if err != nil || entry.DeliveryIssue != nil {
		t.Fatal("uncommitted issue visible", entry, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_deferral`); err != nil {
		t.Fatal(err)
	}
	if err = m.DeferConversationDelivery(scope, c.ID, input.ID, issue); err != nil {
		t.Fatal(err)
	}
	c, _ = m.Conversation(scope, c.ID)
	if err = m.DeferConversationDelivery(scope, c.ID, input.ID, issue); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := m.Conversation(scope, c.ID)
	if unchanged.Revision != c.Revision {
		t.Fatal("identical deferral emitted another revision")
	}
	if _, err = r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig()); err != nil {
		t.Fatal(err)
	}
	if err = m.DeferConversationDelivery(scope, c.ID, input.ID, issue); !errors.Is(err, ErrRunConflict) {
		t.Fatal("deferral overwrote delivered run", err)
	}
}
