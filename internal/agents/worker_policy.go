package agents

import "github.com/takuphilchan/offgrid-llm/pkg/api"

// Absence is the original canonical policy. Keep this version's defaults
// stable so old work never acquires different limits after a service upgrade.
func defaultWorkerPolicyV1() api.WorkerPolicy {
	return api.WorkerPolicy{Version: 1, MaxSessions: 4, MaxActive: 2, MaxDepth: 1, InitialModelCalls: 4}
}

func resolvedWorkerPolicy(task *Task) api.WorkerPolicy {
	if task != nil && task.Config.WorkerPolicy != nil {
		return *task.Config.WorkerPolicy
	}
	return defaultWorkerPolicyV1()
}

func validWorkerPolicy(policy *api.WorkerPolicy) bool {
	return policy == nil || policy.Version == 1 && policy.MaxSessions >= 1 && policy.MaxSessions <= 32 && policy.MaxActive >= 1 && policy.MaxActive <= policy.MaxSessions && policy.MaxActive <= 8 && policy.MaxDepth == 1 && policy.InitialModelCalls >= 1 && policy.InitialModelCalls <= 50
}

// Storage bounds are separate from the number of sessions. A follow-up creates
// another run under a session and must not be mistaken for another specialist.
func childHistoryLimit(task *Task) int {
	if task.WorkID != "" {
		return 128
	}
	return 8
}

func workerSessionCount(task *Task) int {
	seen := map[string]bool{}
	for _, id := range task.PreviousWorkerIDs {
		seen[id] = true
	}
	for _, link := range task.ChildHistory {
		id := link.WorkerID
		if id == "" {
			id = link.ID
		}
		seen[id] = true
	}
	return len(seen)
}
