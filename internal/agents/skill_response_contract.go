package agents

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Read only the retained successful exchanges, matched to immutable task pins.
// Tool text cannot create a contract and a new import cannot alter an old job.
func activeSkillContracts(task *Task) ([]agentcontext.ResponseContract, error) {
	var contracts []agentcontext.ResponseContract
	seen := map[string]bool{}
	for index := range retainedSkillGroups(task) {
		var loaded struct {
			Skill AgentSkill `json:"skill"`
		}
		if json.Unmarshal([]byte(task.Checkpoint.Messages[index+1].StringContent()), &loaded) != nil {
			continue
		}
		for _, pinned := range task.Config.SkillMetadata {
			if pinned.SkillReference != loaded.Skill.SkillReference || pinned.Draft || pinned.ResponseContract == nil {
				continue
			}
			if loaded.Skill.ResponseContract == nil || *loaded.Skill.ResponseContract != *pinned.ResponseContract || pinned.ResponseContract.Validate() != nil {
				return nil, errors.New("loaded skill response contract does not match its reviewed revision")
			}
			if !seen[pinned.ID] {
				contracts = append(contracts, *pinned.ResponseContract)
				seen[pinned.ID] = true
			}
		}
	}
	return contracts, nil
}

// Called only after ordinary effect/workflow verification. Correction may
// change the answer, never execute another tool or replay earlier mutations.
func (r *Runner) repairSkillAnswer(task *Task, answer string) (bool, error) {
	contracts, err := activeSkillContracts(task)
	if err != nil {
		task.ErrorCode = "skill_response_contract_invalid"
		_, err = r.fail(task, err.Error())
		return true, err
	}
	valid := true
	for _, contract := range contracts {
		valid = valid && contract.Matches(answer)
	}
	if valid {
		return false, nil
	}
	cp := task.Checkpoint
	cp.SkillAnswerRepairs++
	feedback := "The proposed answer did not match the complete response format of a loaded reviewed skill. Correct only the answer to satisfy all output-format requirements in the already loaded instructions. All earlier work and evidence remain saved. No tools are available during this correction; do not repeat actions or claim new effects."
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "response_contract_rejected", Content: feedback, Timestamp: time.Now().UTC()})
	if cp.SkillAnswerRepairs > 2 {
		task.ErrorCode = "skill_response_contract_limit"
		_, err := r.fail(task, "The answer did not satisfy the reviewed skill response format after two corrections. Earlier work remains saved; no tools were replayed.")
		return true, err
	}
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "system", Content: feedback})
	return true, r.persist(task)
}
