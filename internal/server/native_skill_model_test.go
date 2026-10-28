package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Real local-model check, isolated service stores and no host grants. File/tool
// success alone cannot pass: require the requested format and a bounded load.
func TestNativeSkillInstalledModel(t *testing.T) {
	runNativeSkillInstalledModel(t, false)
}

func TestNativeContractSkillInstalledModel(t *testing.T) {
	runNativeSkillInstalledModel(t, true)
}

func runNativeSkillInstalledModel(t *testing.T, contracted bool) {
	if os.Getenv("OFFGRID_TEST_NATIVE_SCOPE") == models.NativeWorkspaceContractScope && !contracted {
		t.Fatal("v2 skill case requires the separately reviewed contract fixture")
	}
	engine, model := installedConversationEngine(t)
	adapter := os.Getenv("OFFGRID_TEST_DECISION_ADAPTER")
	if adapter != "" && adapter != "constrained" && adapter != "native" {
		t.Fatal("unknown test adapter")
	}
	t.Logf("decision_adapter=%s (test only; production unchanged)", adapter)
	s := profileFixture(t)
	s.config.EnableExecutionPreview = true
	s.executionHosts = execution.NewHosts()
	// Admission metadata is isolated; inference still goes to the explicitly
	// selected installed model, never these fixture bytes.
	if filepath.Base(model) != model {
		t.Fatal("invalid fixture model name")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, model+".gguf"), []byte("test-only admission metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	s.registry = models.NewRegistry(dir)
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	fixture := "testdata/skills/checklist/SKILL.md"
	if contracted {
		fixture = "testdata/skills/checklist-contract/SKILL.md"
	}
	body, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	skill, err := s.agentManager.ImportSkill(s.workspaceID, "local-admin", "", 0, []agentcontext.SkillFile{{Path: "SKILL.md", Content: body}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if contracted && skill.ResponseContract == nil {
		t.Fatal("reviewed fixture contract missing")
	}
	t.Logf("skill_fixture_digest=%s contracted=%t", skill.Digest, contracted)
	if contracted {
		t.Log("component check only: reviewed fixture is not the frozen native-workspace-v1 skill case")
	}
	settings := agents.ProfileSettings{Name: "Skill regression", Model: model, MaxIterations: 6}
	if path := os.Getenv("OFFGRID_TEST_PROFILE_FIXTURE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var pinned agents.AgentProfile
		if json.Unmarshal(data, &pinned) != nil || pinned.Settings.Model != model || len(pinned.Settings.Delegates) > 0 || len(pinned.Settings.Knowledge) > 0 {
			t.Fatal("unsupported isolated profile fixture")
		}
		settings = pinned.Settings
		t.Log("using supplied profile settings in isolated storage; no installed profile changed")
	}
	settings.Skills = []agentcontext.SkillReference{skill.SkillReference}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, settings)
	if err != nil {
		t.Fatal(err)
	}
	req := jobSubmitRequest{Prompt: "Use offgrid-checklist-test to make a checklist for preparing a demo video.", ProfileID: profile.ID, ProfileRevision: profile.Revision, ExecutionRequestVersion: 1}
	w := httptest.NewRecorder()
	cfg, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok {
		t.Fatal("profile admission", w.Code, w.Body.String())
	}
	cfg.ContextWindow, cfg.MaxTokens = 8192, 1024
	cfg.MaxIterations = 6
	applyNativeWorkspaceTestScope(t, &cfg, adapter)
	started := time.Now()
	runner := agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		zero, max, no := float32(0), 1024, false
		request := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max, SingleStepTools: adapter != "native", ParallelToolCalls: &no}
		acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
		err := engine.ChatCompletionStreamRaw(ctx, request, func(raw json.RawMessage) error { _, _, err := acc.add(raw); return err })
		if err != nil {
			return nil, err
		}
		return acc.response(), nil
	})
	defer runner.Shutdown(context.Background())
	task, err := runner.Create(req.Prompt, model, "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, runErr := runner.Continue(ctx, task.ID, task.Actor, "start", "", false)
	if result == nil {
		t.Fatal(runErr)
	}
	loads := 0
	if contracted {
		for _, message := range result.Checkpoint.Messages {
			if message.Role == "assistant" && message.StringContent() != "" {
				t.Logf("isolated_contract_answer=%q", message.StringContent())
			}
		}
	}
	for _, step := range result.Steps {
		t.Logf("tool=%s type=%s args=%s", step.ToolName, step.Type, step.ToolArgs)
		if step.ToolName == "skill_load" && step.Type == "action" {
			loads++
		} else if step.ToolName != "" && step.ToolName != "task_plan" {
			t.Errorf("unexpected tool %s", step.ToolName)
		}
	}
	t.Logf("model=%s contracted=%t status=%s elapsed_ms=%d loads=%d answer_repairs=%d answer=%q error=%q", model, contracted, result.Status, time.Since(started).Milliseconds(), loads, result.Checkpoint.SkillAnswerRepairs, result.Result, result.Error)
	if runErr != nil || result.Status != agents.TaskCompleted || loads != 1 || !validChecklistFixtureAnswer(result.Result) {
		t.Fatal("skill usage failed independent output checks", runErr)
	}
}

// Only this deliberately fixed fixture uses this oracle. Production never
// rewrites answers to fit a skill. A checkbox containing "Next step:" must not
// count as the separate closing sentence requested by the skill.
func validChecklistFixtureAnswer(answer string) bool {
	var lines []string
	for _, line := range strings.Split(answer, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 5 || lines[0] != "OFFGRID SKILL CHECK: 7K2" || !strings.HasPrefix(lines[4], "Next step: ") || strings.TrimSpace(strings.TrimPrefix(lines[4], "Next step:")) == "" {
		return false
	}
	for _, line := range lines[1:4] {
		if !strings.HasPrefix(line, "- [ ] ") || strings.TrimSpace(strings.TrimPrefix(line, "- [ ]")) == "" || strings.Contains(line, "Next step:") {
			return false
		}
	}
	return true
}

func TestChecklistFixtureOracleRejectsMergedClosingSentence(t *testing.T) {
	valid := "OFFGRID SKILL CHECK: 7K2\n- [ ] Prepare a script.\n- [ ] Test the microphone.\n- [ ] Record a sample.\nNext step: Choose the topic."
	if !validChecklistFixtureAnswer(valid) || !validChecklistFixtureAnswer(strings.ReplaceAll(valid, "\n", "\r\n\r\n")) {
		t.Fatal("valid shape rejected")
	}
	for _, invalid := range []string{
		"Introduction\n" + valid,
		strings.Replace(valid, "7K2\n", "7K2 extra\n", 1),
		"OFFGRID SKILL CHECK: 7K2\n- [ ] Prepare a script.\n- [ ] Test the microphone.\n- [ ] Next step: Start recording.",
		strings.Replace(valid, "Choose the topic.", "", 1),
		strings.Replace(valid, "Test the microphone.", "", 1),
	} {
		if validChecklistFixtureAnswer(invalid) {
			t.Fatal("invalid shape accepted", invalid)
		}
	}
}

func TestReviewedChecklistContractPreservesWhitespaceAcceptance(t *testing.T) {
	body, err := os.ReadFile("testdata/skills/checklist-contract/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	cases, _ := models.NativeWorkspaceCasesForScope(models.NativeWorkspaceContractScope)
	if !strings.Contains(cases[len(cases)-1].Setup, hex.EncodeToString(digest[:])) {
		t.Fatal("reviewed skill bytes changed without a new frozen suite identity")
	}
	bundle, err := agentcontext.Parse([]agentcontext.SkillFile{{Path: "SKILL.md", Content: body}})
	if err != nil || bundle.ResponseContract == nil {
		t.Fatal("missing reviewed contract", err)
	}
	answer := "OFFGRID SKILL CHECK: 7K2\n- [ ] Prepare café 日本語 script.\n- [ ] Test the microphone.\n- [ ] Record a sample.\nNext step: Choose the topic."
	for _, valid := range []string{answer, strings.ReplaceAll(answer, "\n", "\r\n\r\n"), answer + "\n"} {
		if !bundle.ResponseContract.Matches(valid) || !validChecklistFixtureAnswer(valid) {
			t.Fatal("valid formatting rejected", valid)
		}
	}
	for _, invalid := range []string{"Introduction\n" + answer, strings.Split(answer, "\nNext step:")[0], answer + "\nUnexpected extra line", strings.Replace(answer, "Choose the topic.", "   ", 1), strings.Replace(answer, "Test the microphone.", "   ", 1)} {
		if bundle.ResponseContract.Matches(invalid) {
			t.Fatal("invalid complete answer accepted", invalid)
		}
	}
}
