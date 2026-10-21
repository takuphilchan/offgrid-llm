package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const collaborationQualificationPrompt = "Have the researcher gather five supported findings about the topic in my attached test document. Then have the writer turn those findings into a 300-word briefing. Return the briefing here with references and unresolved uncertainties. Do not open applications or save files."

// Fictional, public test material, not assertions about real destinations. These
// are admission-time retrieved excerpts: this fixture does not qualify retrieval,
// ingestion, web research, or a user's existing attachments/profile configuration.
const collaborationQualificationSource = `Document: Synthetic visitor briefing test document
Topic: planning a visit to fictional Alder Island. All locations and figures are fictional.
[1] Cedar transit note: The Cedar shuttle serves 14 stops.
[2] Lumen ticket note: Lumen garden admission costs 9 credits.
[3] Brindle timetable note: The Brindle ferry crossing takes 37 minutes.
[4] Tova reopening note: Tova museum reopened on 12 April 2026.
[5] Sable route note: The Sable trail is 6.4 kilometres long.
Unresolved: no evidence establishes current weather disruption or accessibility arrangements.
These five notes are the entire source set. Do not infer opening hours, real currency, booking availability, or additional facts.`

const collaborationQualificationFindings = `1. The Cedar shuttle serves 14 stops. [1]
2. Lumen garden admission costs 9 credits. [2]
3. The Brindle ferry crossing takes 37 minutes. [3]
4. Tova museum reopened on 12 April 2026. [4]
5. The Sable trail is 6.4 kilometres long. [5]
Uncertainties: current weather disruption and accessibility arrangements are unknown.`

const collaborationQualificationFormat = "Use these Markdown headings: ## Briefing, ## References, ## Uncertainties. The Briefing body must contain exactly 300 whitespace-separated words, excluding standalone numbered source citations such as [1]; headings, references and uncertainties are outside that count. Cite each of the five findings beside its claim. Preserve source labels and distinguish uncertainty from established facts. Do not claim to have opened applications or saved files."

const collaborationQualificationBriefing = `## Briefing
This briefing concerns fictional Alder Island and draws only on the supplied visitor notes. It is a planning summary, not a verified itinerary or a statement about any real destination. The evidence supports five specific findings, while leaving important practical questions unresolved for anyone considering the suggested activities.

The Cedar shuttle serves 14 stops. [1] That establishes the stated number of stops, but does not establish their locations, service frequency, operating hours, or connections. A visitor should therefore treat the shuttle as a transport option requiring further practical information rather than assume that it connects every place discussed in these notes.

Lumen garden admission costs 9 credits. [2] The source provides a price in fictional credits, without identifying concessions, payment arrangements, or booking conditions. This amount should not be converted into a real currency or treated as evidence that tickets are currently available. Those details would require additional evidence before making commitments.

The Brindle ferry crossing takes 37 minutes. [3] This figure describes the crossing itself. It does not establish waiting time, boarding procedures, departures, or the total journey between accommodation and an attraction. Planning should distinguish the supported crossing duration from the broader travel time, which remains unknown from this document.

Tova museum reopened on 12 April 2026. [4] That historical statement does not independently confirm present operating hours or current admission arrangements. The Sable trail is 6.4 kilometres long. [5] Its length alone does not establish walking time, terrain, accessibility, or suitability for a particular visitor. Both activities require further checks before a definite itinerary can be proposed.

The strongest conclusion is deliberately limited: these notes offer useful starting points, not a complete travel plan. Current weather disruption and accessibility arrangements remain unresolved. Keep confirmed facts separate from assumptions, seek appropriate current information, and avoid presenting this fictional exercise as real travel advice.

## References
[1] Cedar transit note, Synthetic visitor briefing test document.
[2] Lumen ticket note, Synthetic visitor briefing test document.
[3] Brindle timetable note, Synthetic visitor briefing test document.
[4] Tova reopening note, Synthetic visitor briefing test document.
[5] Sable route note, Synthetic visitor briefing test document.

## Uncertainties
Current weather disruption and accessibility arrangements remain unknown; the supplied notes contain no supporting evidence for either.`

// The only executor is deliberately empty. Runtime plan/history/delegation
// metadata remains supported by Runner, but no file, network or desktop action
// can cross this boundary, even if a generated proposal asks for one.
type collaborationNoEffectsTools struct{ dispatches atomic.Int32 }

func (*collaborationNoEffectsTools) GetTools() []api.Tool { return nil }
func (*collaborationNoEffectsTools) Capability(string) (capabilities.Descriptor, bool) {
	return capabilities.Descriptor{}, false
}
func (*collaborationNoEffectsTools) Authorize(context.Context, string, json.RawMessage, agents.ToolExecution) error {
	return capabilities.ErrDenied
}
func (f *collaborationNoEffectsTools) ExecuteWithPolicy(context.Context, string, json.RawMessage, agents.ToolExecution) (string, error) {
	f.dispatches.Add(1)
	return "", capabilities.ErrDenied
}

func collaborationQualifiedConfig(t *testing.T, manager *agents.Manager, model string) agents.AgentConfig {
	t.Helper()
	const actor, workspace, document = "fixture-actor", "fixture-workspace", "fixture-public-document"
	makeProfile := func(name, instructions string, knowledge []string, delegates []string, iterations int) *agents.AgentProfile {
		profile, err := manager.SaveProfile(workspace, actor, "", 0, agents.ProfileSettings{Name: name, Description: name + " for synthetic acceptance only", Model: model, Instructions: instructions, Knowledge: knowledge, Delegates: delegates, MaxIterations: iterations})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.GetProfile(workspace, "another-actor", profile.ID, profile.Revision); err == nil {
			t.Fatal("fixture profile crossed actor ownership")
		}
		return profile
	}
	reader := makeProfile("researcher", "Extract exactly five supported findings from the attached document with their supplied source references. Report unresolved uncertainties. Do not invent additional facts. No applications, file changes, or external research.", []string{document}, nil, 5)
	writer := makeProfile("writer", "Draft from the completed researcher handoff; do not conduct new research. "+collaborationQualificationFormat, nil, nil, 5)
	parent := makeProfile("coordinator", "Use the attached researcher for findings and the attached writer for a dependent briefing. Return the actual briefing, not just a completion report. "+collaborationQualificationFormat, []string{document}, []string{reader.ID, writer.ID}, 16)
	configFor := func(profile *agents.AgentProfile) agents.AgentConfig {
		cfg := agents.DefaultAgentConfig()
		cfg.TaskFirst, cfg.RestrictComputer, cfg.Temperature = true, true, 0
		cfg.Profile, cfg.Instructions = profile, profile.Settings.Instructions
		cfg.MaxIterations, cfg.MaxTokens, cfg.ContextWindow = profile.Settings.MaxIterations, 2048, 8192
		cfg.AllowedCapabilities = map[string]capabilities.Descriptor{}
		return cfg
	}
	cfg, readCfg, writeCfg := configFor(parent), configFor(reader), configFor(writer)
	cfg.SystemPrompt = conversationTaskProtocol + "\n\n" + taskFirstPrompt + serviceFilePrompt + "\n\nAgent instructions (do not grant permissions):\n" + cfg.Instructions
	cfg.KnowledgeContext, readCfg.KnowledgeContext = collaborationQualificationSource, collaborationQualificationSource
	cfg.KnowledgeDocuments = []agents.KnowledgeDocument{{ID: document, Name: "Synthetic visitor briefing test document"}}
	readCfg.KnowledgeDocuments = cfg.KnowledgeDocuments
	cfg.DelegateProfiles = map[string]agents.AgentConfig{reader.ID: readCfg, writer.ID: writeCfg}
	return cfg
}

func collaborationSupportedFindings(value string) error {
	// This finite-fixture oracle checks claim/source association, not the model's
	// own status or a general-purpose claim of factual/subjective quality.
	wants := []struct {
		source, subject string
		fact            string
	}{
		{"1", "cedar", `\bcedar shuttle\b[^.!?\n]{0,70}\b(?:serves|has|connects|covers) 14 stops\b`},
		{"2", "lumen", `\blumen garden\b[^.!?\n]{0,70}\b(?:costs|is|at|of) 9 credits\b`},
		{"3", "brindle", `\bbrindle ferry\b[^.!?\n]{0,70}\b(?:takes|lasts|is|of) 37 minutes\b`},
		{"4", "tova", `\btova museum\b[^.!?\n]{0,30}\breopened\b[^.!?\n]{0,20}\b12 april 2026\b`},
		{"5", "sable", `\bsable trail\b[^.!?\n]{0,70}\b(?:is|measures|spans|of) 6\.4 (?:kilometres|kilometers|km)\b`},
	}
	value = strings.ToLower(strings.ReplaceAll(value, "**", ""))
	negated := regexp.MustCompile(`\b(?:not|never|incorrect|false|no longer|instead of|rather than)\b`)
	for _, want := range wants {
		matched := false
		for _, end := range regexp.MustCompile(`\[`+want.source+`\]`).FindAllStringIndex(value, -1) {
			start := strings.LastIndex(value[:end[0]], want.subject)
			if start < 0 || end[0]-start > 320 {
				continue
			}
			claim := value[start:end[0]]
			// Check the sentence prefix too, so "It is false that Cedar ..."
			// cannot turn a negated or incorrect figure into supporting evidence.
			prefix := value[max(0, start-100):start]
			if boundary := strings.LastIndexAny(prefix, ".!?\n"); boundary >= 0 {
				prefix = prefix[boundary+1:]
			}
			matched = matched || (regexp.MustCompile(want.fact).MatchString(claim) && !negated.MatchString(prefix+claim))
		}
		if !matched {
			return fmt.Errorf("missing supported %s finding with adjacent source [%s]", want.subject, want.source)
		}
	}
	return nil
}

func collaborationBriefingOracle(value string) error {
	start, refs, unknown := strings.Index(value, "## Briefing\n"), strings.Index(value, "## References\n"), strings.Index(value, "## Uncertainties\n")
	if start < 0 || refs <= start || unknown <= refs {
		return fmt.Errorf("missing ordered briefing, references and uncertainty sections")
	}
	body := strings.TrimSpace(value[start+len("## Briefing\n") : refs])
	words := 0
	for _, word := range strings.Fields(body) {
		if !regexp.MustCompile(`^\[[1-5]\]$`).MatchString(word) {
			words++
		}
	}
	if words != 300 {
		return fmt.Errorf("briefing contains %d words, expected exactly 300 excluding citations", words)
	}
	if err := collaborationSupportedFindings(body); err != nil {
		return err
	}
	for _, pair := range []struct{ id, title string }{{"1", "Cedar transit note"}, {"2", "Lumen ticket note"}, {"3", "Brindle timetable note"}, {"4", "Tova reopening note"}, {"5", "Sable route note"}} {
		if !strings.Contains(value[refs:unknown], "["+pair.id+"] "+pair.title) {
			return fmt.Errorf("source [%s] is missing or mislabeled", pair.id)
		}
	}
	uncertainty := strings.ToLower(value[unknown:])
	for _, subject := range []string{"weather", "accessibility"} {
		found := false
		for _, clause := range strings.FieldsFunc(uncertainty, func(r rune) bool { return strings.ContainsRune(".;!?\n,", r) }) {
			found = found || (strings.Contains(clause, subject) && (strings.Contains(clause, "unknown") || strings.Contains(clause, "unresolved") || strings.Contains(clause, "not established") || strings.Contains(clause, "not known")))
		}
		if !found {
			return fmt.Errorf("missing unresolved %s limitation", subject)
		}
	}
	return nil
}

func runCollaborationBriefingFixture(t *testing.T, model string, caller agents.RunCaller, count func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error)) {
	t.Helper()
	store := t.TempDir()
	manager := agents.NewManagerWithPersistence(nil, nil, nil, store)
	cfg := collaborationQualifiedConfig(t, manager, model)
	tools := &collaborationNoEffectsTools{}
	var boundaryMu sync.Mutex
	boundaryError := ""
	runner := agents.NewRunner(manager, tools, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, menu []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" && task.Config.Profile.Settings.Name == "writer" {
			found := false
			for _, message := range messages {
				if strings.Contains(message.StringContent(), "Results from completed dependency tasks") && collaborationSupportedFindings(message.StringContent()) == nil {
					found = true
				}
			}
			if !found {
				boundaryMu.Lock()
				boundaryError = "writer did not receive five supported researcher findings; researcher result: " + task.Result
				boundaryMu.Unlock()
				return nil, fmt.Errorf("writer missing actual research dependency")
			}
		}
		return caller(ctx, task, messages, menu)
	})
	runner.CountContext = count
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	defer runner.Shutdown(context.Background())
	runner.StartCoordinator(ctx)
	root, err := runner.Create(collaborationQualificationPrompt, model, "fixture-actor", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Continue(ctx, root.ID, root.Actor, "start", "", true); err != nil {
		t.Fatal(err)
	}
	var result *agents.Task
	for {
		result, _ = manager.GetTask(root.ID)
		if result.Status != agents.TaskPending && result.Status != agents.TaskRunning && result.Status != agents.TaskChildren {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("collaboration timed out without replay", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	boundaryMu.Lock()
	boundary := boundaryError
	boundaryMu.Unlock()
	if boundary != "" {
		t.Fatal(boundary)
	}
	if result.Status != agents.TaskCompleted {
		t.Fatalf("collaboration did not complete: status=%s code=%s error=%s", result.Status, result.ErrorCode, result.Error)
	}
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Reload after worker shutdown: an in-memory result alone cannot establish
	// durable root/child identity, dependency attachment or stored profile pins.
	manager = agents.NewManagerWithPersistence(nil, nil, nil, store)
	result, _ = manager.GetTask(root.ID)
	if manager.StorageError() != nil || result == nil || result.Status != agents.TaskCompleted {
		t.Fatal("completed root did not survive reload", manager.StorageError())
	}
	if err := collaborationBriefingOracle(result.Result); err != nil {
		t.Fatalf("independent output check failed: %v\nSynthetic output:\n%s", err, result.Result)
	}
	if len(result.ChildHistory) != 2 || len(manager.ListTasks()) != 3 {
		t.Fatalf("expected exactly one root and two children: children=%d tasks=%d", len(result.ChildHistory), len(manager.ListTasks()))
	}
	var researchKey string
	roles := map[string]int{}
	for _, child := range result.ChildHistory {
		saved, ok := manager.GetTask(child.ID)
		if !ok || saved.ParentID != root.ID || saved.Actor != root.Actor || saved.Status != agents.TaskCompleted || saved.Config.Profile == nil || saved.Config.Profile.Revision != 1 || saved.Config.ComputerSession != "" || len(saved.ChildHistory) != 0 {
			t.Fatal("child identity, scope or completion invariant failed")
		}
		pinned, approved := cfg.DelegateProfiles[saved.Config.Profile.ID]
		if !approved || pinned.Profile.Digest != saved.Config.Profile.Digest || child.Spec.ProfileID != saved.Config.Profile.ID || len(child.Spec.Tools) != 0 {
			t.Fatal("child profile or tool scope differs from approved fixture resources")
		}
		roles[saved.Config.Profile.Settings.Name]++
		if saved.Config.Profile.Settings.Name == "researcher" {
			researchKey = child.Spec.Key
			if err := collaborationSupportedFindings(saved.Result); err != nil {
				t.Fatal("researcher findings", err)
			}
		}
	}
	if researchKey == "" || roles["researcher"] != 1 || roles["writer"] != 1 {
		t.Fatal("expected one researcher and one writer", roles)
	}
	for _, child := range result.ChildHistory {
		saved, _ := manager.GetTask(child.ID)
		if saved.Config.Profile.Settings.Name == "writer" {
			if len(child.Spec.DependsOn) != 1 || child.Spec.DependsOn[0] != researchKey || !saved.DependenciesAttached {
				t.Fatal("writer was not dependent on the researcher")
			}
			if err := collaborationBriefingOracle(saved.Result); err != nil {
				t.Fatalf("writer did not independently produce the requested briefing: %v\nSynthetic writer output:\n%s", err, saved.Result)
			}
		}
	}
	if result.Checkpoint.Iteration+result.Checkpoint.ReservedChildIterations > cfg.MaxIterations || tools.dispatches.Load() != 0 {
		t.Fatal("root budget or no-effects boundary violated")
	}
	for _, task := range manager.ListTasks() {
		for _, step := range task.Steps {
			if step.Type == "action" || step.ToolName == "task_save_artifact" {
				t.Fatal("unexpected external action or artifact creation")
			}
		}
	}
	t.Logf("verified synthetic collaboration: root=1 children=2 root_iterations=%d reserved_child_iterations=%d briefing_words=300 supported_findings=5 external_dispatches=0", result.Checkpoint.Iteration, result.Checkpoint.ReservedChildIterations)
}

func TestCollaborationBriefingFixture(t *testing.T) {
	runCollaborationBriefingFixture(t, "fixture-model", func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		content := collaborationQualificationBriefing
		if task.ParentID == "" && len(task.ChildHistory) == 0 {
			var reader, writer string
			for id, profile := range task.Config.DelegateProfiles {
				if profile.Profile.Settings.Name == "researcher" {
					reader = id
				} else {
					writer = id
				}
			}
			arguments, _ := json.Marshal(map[string]any{"tasks": []agents.ChildSpec{{Key: "research", Goal: "Extract five source-supported findings with citations and uncertainties from your attached test document.", Tools: []string{}, DependsOn: []string{}, ProfileID: reader}, {Key: "briefing", Goal: collaborationQualificationFormat, Tools: []string{}, DependsOn: []string{"research"}, ProfileID: writer}}})
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "fixture-delegation", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: string(arguments)}}}}}}}, nil
		}
		if task.Config.Profile.Settings.Name == "researcher" {
			content = collaborationQualificationFindings
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: content}}}}, nil
	}, nil)
}

func TestCollaborationBriefingOracleRejectsFalseSuccess(t *testing.T) {
	if err := collaborationBriefingOracle(collaborationQualificationBriefing); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		"Completed. Five findings and a 300-word briefing are ready.",
		strings.Replace(collaborationQualificationBriefing, "14 stops", "15 stops", 1),
		strings.Replace(collaborationQualificationBriefing, "14 stops", "114 stops", 1),
		strings.Replace(collaborationQualificationBriefing, "9 credits", "19 credits", 1),
		strings.Replace(collaborationQualificationBriefing, "37 minutes", "137 minutes", 1),
		strings.Replace(collaborationQualificationBriefing, "6.4 kilometres", "16.4 kilometres", 1),
		strings.Replace(collaborationQualificationBriefing, "museum reopened", "museum closed", 1),
		strings.Replace(collaborationQualificationBriefing, "stops. [1]", "stops. [2]", 1),
		strings.Replace(collaborationQualificationBriefing, "## Briefing\n", "## Briefing\nExtra ", 1),
		strings.Replace(collaborationQualificationBriefing, "[3] Brindle timetable note", "[3] Invented source", 1),
		strings.Split(collaborationQualificationBriefing, "## Uncertainties")[0] + "## Uncertainties\nWeather is clear; accessibility is unknown.",
		strings.Split(collaborationQualificationBriefing, "## Uncertainties")[0],
	} {
		if collaborationBriefingOracle(input) == nil {
			t.Fatal("invalid output accepted")
		}
	}
	for _, input := range []string{
		strings.Replace(collaborationQualificationFindings, "serves 14 stops", "does not serve 14 stops", 1),
		strings.Replace(collaborationQualificationFindings, "The Cedar", "It is false that the Cedar", 1),
	} {
		if collaborationSupportedFindings(input) == nil {
			t.Fatal("negated finding accepted")
		}
	}
}

// Opt-in: actual installed model, actual durable runner/coordinator, synthetic
// retrieved source only. No generation retry, model download or desktop access.
func TestCollaborationBriefingInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicit installed loopback model; synthetic attachment and isolated profiles only")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() || u.Path != "/v1/chat/completions" {
		t.Fatal("requires literal loopback completion endpoint")
	}
	engine := inference.NewLlamaHTTPEngine(strings.TrimSuffix(endpoint, "/v1/chat/completions"))
	for repetition := 1; repetition <= 3; repetition++ {
		if !t.Run(fmt.Sprintf("repeat-%d", repetition), func(t *testing.T) {
			var inferenceMu sync.Mutex // One generation at a time on unqualified local hardware.
			calls, totalTokens, peakTokens := 0, 0, 0
			var generationTime, countTime time.Duration
			request := func(task *agents.Task, messages []api.ChatMessage, tools []api.Tool) *api.ChatCompletionRequest {
				zero, maxTokens := float32(0), task.Config.MaxTokens
				choice := interface{}("auto")
				if agents.DelegationRequired(task) {
					choice = map[string]any{"type": "function", "function": map[string]any{"name": "delegate_tasks"}}
				}
				req := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: choice, Temperature: &zero, MaxTokens: &maxTokens, StreamOptions: &api.StreamOptions{IncludeUsage: true}}
				configureComputerRequest(req, task) // Same TaskFirst adapter as production agent_stream.go, not native instructions.
				return req
			}
			counter := func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
				inferenceMu.Lock()
				defer inferenceMu.Unlock()
				start := time.Now()
				n, err := engine.CountPromptTokens(ctx, request(task, messages, tools))
				countTime += time.Since(start)
				return n, err
			}
			caller := func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				inferenceMu.Lock()
				defer inferenceMu.Unlock()
				req := request(task, messages, tools)
				started := time.Now()
				acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
				err := engine.ChatCompletionStreamRaw(ctx, req, func(data json.RawMessage) error { _, _, err := acc.add(data); return err })
				calls++
				generationTime += time.Since(started)
				if err != nil {
					return nil, err
				}
				resp := acc.response()
				if err := validateAgentDecision(task, req, resp); err != nil {
					return nil, err
				}
				if resp.Usage.PromptTokens <= 0 || resp.Usage.PromptTokens+task.Config.MaxTokens+512 > task.Config.ContextWindow {
					return nil, fmt.Errorf("missing or out-of-budget runtime context measurement")
				}
				totalTokens += resp.Usage.PromptTokens
				peakTokens = max(peakTokens, resp.Usage.PromptTokens)
				t.Logf("synthetic role=%s call=%d prompt_tokens=%d output_tokens=%d generation_ms=%d", task.Config.Profile.Settings.Name, calls, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, time.Since(started).Milliseconds())
				for _, choice := range resp.Choices {
					for _, call := range choice.Message.ToolCalls {
						t.Logf("synthetic proposal: %s %s", call.Function.Name, call.Function.Arguments)
					}
				}
				return resp, nil
			}
			defer func() {
				t.Logf("OFFGRID_COLLABORATION_METRICS calls=%d prompt_total=%d prompt_peak=%d generation_ms=%d tokenize_ms=%d queue_ms=unmeasured ui_ms=not_applicable", calls, totalTokens, peakTokens, generationTime.Milliseconds(), countTime.Milliseconds())
			}()
			runCollaborationBriefingFixture(t, model, caller, counter)
		}) {
			break
		}
	}
}
