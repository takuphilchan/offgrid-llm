package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const (
	nativeProgressWarning = 3
	nativeProgressLimit   = 5
	nativeProgressVersion = 1
)

// NativeProgressState is bounded task-local bookkeeping, never evidence or
// authority. Only digests are retained; private text remains in the existing
// canonical result. Timestamps and freshly minted observation references cannot
// make an otherwise unchanged read count as execution progress.
type NativeProgressState struct {
	Version   int               `json:"version"`
	Scope     string            `json:"scope"`
	Evidence  map[string]string `json:"evidence"`
	Checks    string            `json:"checks"`
	Unchanged int               `json:"unchanged"`
	Warned    bool              `json:"warned"`
}

// recordNativeReadProgress is called only after a successful canonical result
// and workflow assessment, before their shared persistence boundary. The caller
// must persist these changes before another inference, and return if true.
// Each read already consumed one root iteration; feedback grants no new budget,
// does not execute anything, and never replays a mutation or resumes saved work.
func recordNativeReadProgress(task *Task, descriptor capabilities.Descriptor, name, result string) bool {
	if task == nil || task.Checkpoint == nil || !UsesNativeActionContract(task) || !task.Config.TaskFirst || task.Config.ComputerSession == "" || task.ComputerSessionExpired || task.Status != TaskRunning || descriptor.Name != name || !capabilities.TrustedNativeRead(descriptor) {
		return false
	}
	cp := task.Checkpoint
	// Do not replace uncertainty or queued execution with a progress diagnosis.
	if cp.ExecutingCall != "" || len(cp.Calls) != 0 || cp.ComputerReceipt != nil {
		return false
	}
	semantic, ok := nativeReadSemantics(name, result)
	if !ok {
		return false // Invalid/error results neither count nor clear a stalled stage.
	}
	correction := []string{}
	if len(task.Instructions) > 0 {
		latest := task.Instructions[len(task.Instructions)-1]
		correction = []string{latest.RequestID, latest.Digest}
	}
	// A user correction starts a new task-intent epoch only once applied at a
	// safe boundary. Pending steering and model-authored workflow revisions do
	// not erase accumulated no-progress state.
	scope := nativeProgressDigest([]any{task.Actor, task.Config.ComputerSession, task.Config.ComputerStepOffset, len(task.Instructions), correction})
	state := cp.NativeProgress
	if state == nil || state.Version != nativeProgressVersion || state.Scope != scope || state.Evidence == nil || len(state.Evidence) > 12 {
		state = &NativeProgressState{Version: nativeProgressVersion, Scope: scope, Evidence: map[string]string{}}
		cp.NativeProgress = state
	}
	digest, checks := nativeProgressDigest(semantic), nativeVerifiedProgress(task)
	previous, known := state.Evidence[name]
	state.Evidence[name] = digest
	if !known || previous != digest || state.Checks != checks {
		state.Checks, state.Unchanged, state.Warned = checks, 0, false
		return false
	}
	state.Unchanged++
	if state.Unchanged < nativeProgressWarning {
		return false
	}
	if !state.Warned {
		state.Warned = true
		feedback := "Repeated native reads returned unchanged information without advancing the requested outcome. Use the current observed capabilities to choose the next authorized step, or explain what is blocked. Another identical inspection, catalog lookup or metadata update is not progress. Do not repeat any dispatched input or invent completion; changed targets and genuinely new evidence may still be inspected."
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "user", Content: feedback})
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "progress_notice", Content: feedback, Timestamp: time.Now().UTC()})
	}
	if state.Unchanged < nativeProgressLimit {
		return false
	}
	task.Status, task.ErrorCode = TaskInterrupted, "computer_no_progress"
	task.Error = "Paused because repeated inspections found no new information and the workflow did not advance. Earlier work and verification are preserved; no input was repeated by this recovery. Review the remaining task before explicitly continuing."
	setRunPhase(task, "interrupted", "")
	return true
}

func nativeProgressDigest(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// New declarations, plan edits and read timestamps are not verified progress.
// A changed independently assessed check state is meaningful only when it has
// recorded evidence. Omit titles, IDs and model-authored prose from the digest.
func nativeVerifiedProgress(task *Task) string {
	var states []string
	seen := map[string]bool{}
	if task.Workflow != nil {
		for _, check := range task.Workflow.Checks {
			if len(check.EvidenceSteps) == 0 || check.State == "pending" || check.State == "declared" {
				continue
			}
			digest := nativeProgressDigest([]any{check.Expectation.Kind, check.Expectation.TargetID, check.Expectation.ApplicationID, check.Expectation.Document, check.State})
			if !seen[digest] {
				states, seen[digest] = append(states, digest), true
			}
		}
	}
	sort.Strings(states)
	return nativeProgressDigest(states)
}

func nativeReadSemantics(name, result string) (any, bool) {
	// Native replies already have lower transport bounds. Keep this independent
	// guard for saved fixtures/older records and never retain arbitrary payloads.
	if len(result) == 0 || len(result) > 128<<10 || jsonwire.Validate([]byte(result)) != nil {
		return nil, false
	}
	var data map[string]any
	if json.Unmarshal([]byte(result), &data) != nil || data == nil || data["error"] != nil || data["status"] == "failed" {
		return nil, false
	}
	pick := func(source map[string]any, keys ...string) map[string]any {
		out := map[string]any{}
		for _, key := range keys {
			if value, exists := source[key]; exists {
				out[key] = value
			}
		}
		return out
	}
	observation := func(value any) (map[string]any, bool) {
		v, ok := value.(map[string]any)
		target, targetOK := v["target"].(map[string]any)
		if !ok || !targetOK || target["id"] == nil || target["id"] == "" {
			return nil, false
		}
		return pick(v, "target", "state_digest"), true
	}
	view := func(value map[string]any) (any, bool) {
		observed, ok := observation(value["observation"])
		elements, exists := value["elements"].([]any)
		if !ok || !exists {
			return nil, false
		}
		stable := make([]any, 0, len(elements))
		for _, element := range elements {
			row, ok := element.(map[string]any)
			if !ok {
				return nil, false
			}
			// Element IDs are capture-bound; identity and properties are evidence.
			stable = append(stable, pick(row, "identity", "name", "role", "writable", "invokable", "checkable", "checked", "text", "text_limited"))
		}
		return map[string]any{"observation": observed, "elements": stable, "limited": value["limited"]}, true
	}
	switch name {
	case "computer_observe":
		return view(data)
	case "computer_verify_application":
		v, ok := data["view"].(map[string]any)
		if !ok || data["verified"] != true {
			return nil, false
		}
		stable, valid := view(v)
		return []any{data["application_id"], data["check"], stable}, valid
	case "computer_documents":
		observed, ok := observation(data["observation"])
		documents, docsOK := data["documents"].([]any)
		commands, commandsOK := data["commands"].([]any)
		if !ok || !docsOK || !commandsOK || data["complete"] != true {
			return nil, false
		}
		// The document provider's state digest includes the active binding's
		// fresh observation ID. Use the complete semantic inventory instead.
		delete(observed, "state_digest")
		active, _ := data["active"].(map[string]any)
		stableCommands := make([]any, 0, len(commands))
		for _, command := range commands {
			row, valid := command.(map[string]any)
			if !valid {
				return nil, false
			}
			stableCommands = append(stableCommands, pick(row, "kind", "profile", "revision"))
		}
		return []any{observed, documents, pick(active, "target", "document", "generation", "container", "editor", "revision"), stableCommands}, true
	case "computer_read_text":
		binding, ok := data["binding"].(map[string]any)
		if !ok || binding["target"] == nil || binding["document"] == nil || binding["revision"] == nil || data["text_sha256"] == nil || data["complete"] != true {
			return nil, false
		}
		page, _ := data["page"].(map[string]any)
		return []any{pick(binding, "target", "document", "generation", "container", "editor", "revision"), pick(data, "text_sha256", "bytes", "composition", "selection"), pick(page, "start", "end", "total_scalars", "next", "text", "source_complete")}, true
	case "computer_applications", "computer_find_applications":
		if _, ok := data["applications"].([]any); !ok {
			return nil, false
		}
		return pick(data, "applications", "query", "resolution", "total_matches", "next_offset"), true
	case "computer_windows":
		if _, ok := data["windows"].([]any); !ok {
			return nil, false
		}
		return data["windows"], true
	default:
		// Unreviewed/other read shapes cannot claim progress or trip this guard.
		return nil, false
	}
}
