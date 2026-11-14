package agents

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerPolicyPinsAdmissionConcurrencyAndAllocation(t *testing.T) {
	m, r, scope, conversation := controlFixture(t)
	policy := defaultWorkerPolicyV1()
	policy.MaxSessions, policy.MaxActive, policy.InitialModelCalls = 1, 1, 2
	config := DefaultAgentConfig()
	config.WorkerPolicy = &policy
	command := ConversationCommand{RequestID: "policy-work", TargetID: conversation.ID, ExpectedRevision: conversation.Revision, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Review this"}}}
	receipt, err := r.AdmitConversationWork(scope, command, "fixture-model", config)
	if err != nil {
		t.Fatal(err)
	}
	policy.MaxSessions = 12 // Mutation of the caller's preferences cannot change the pin.
	root, _ := m.GetTask(receipt.Work.RunID)
	if root.Config.WorkerPolicy.MaxSessions != 1 || receipt.Work.WorkerPolicy.MaxSessions != 1 {
		t.Fatal("policy was not copied at admission")
	}
	if budget, err := childDecisionBudget(root, 1); err != nil || budget != 2 {
		t.Fatal("initial allocation ignored policy", budget, err)
	}
	child := &Task{ID: "child"}
	if slots := workerConcurrencySlots(root, map[string]*Task{"child": child}, map[string]context.CancelFunc{"child": func() {}}, 8); slots != 0 {
		t.Fatal("active limit ignored", slots)
	}
	root.Status = TaskRunning
	call := api.ToolCall{ID: "too-many", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"one","goal":"One","depends_on":[],"tools":[]},{"key":"two","goal":"Two","depends_on":[],"tools":[]}]}`}}
	if err := r.beginDelegation(root, call); err == nil {
		t.Fatal("session ceiling bypassed")
	}
	if len(m.ListTasks()) != 1 {
		t.Fatal("rejected assignment created children")
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	stored, _ := reloaded.GetTask(root.ID)
	if *stored.Config.WorkerPolicy != *root.Config.WorkerPolicy {
		t.Fatal("restart changed worker policy")
	}
}

func TestWorkerPolicyUpgradeAndInvalidVersion(t *testing.T) {
	m, r, scope, conversation := controlFixture(t)
	work := admitControlTestWork(t, r, scope, conversation)
	if _, err := m.updateTask(work.RunID, func(task *Task) error { task.Config.WorkerPolicy = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE agent_schema SET version=23`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	stored, _ := reloaded.GetTask(work.RunID)
	if stored.Config.WorkerPolicy != nil || resolvedWorkerPolicy(stored) != defaultWorkerPolicyV1() {
		t.Fatal("legacy policy changed")
	}
	db, err = reloaded.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw []byte
	if err := db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		From, To int
		Backup   string
	}
	if json.Unmarshal(raw, &manifest) != nil || manifest.From != 23 || manifest.To != taskSchemaVersion || manifest.Backup == "" {
		t.Fatal("missing upgrade backup", manifest)
	}
	for _, mutate := range []func(*api.WorkerPolicy){func(p *api.WorkerPolicy) { p.Version = 2 }, func(p *api.WorkerPolicy) { p.MaxDepth = 2 }, func(p *api.WorkerPolicy) { p.MaxActive = p.MaxSessions + 1 }, func(p *api.WorkerPolicy) { p.InitialModelCalls = 0 }} {
		policy := defaultWorkerPolicyV1()
		mutate(&policy)
		stored.Config.WorkerPolicy = &policy
		if validateStoredTask(stored) == nil {
			t.Fatal("invalid policy accepted", policy)
		}
	}
}
