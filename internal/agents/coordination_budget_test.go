package agents

import "testing"

func TestChildDecisionBudgetReservesParentVerificationAndAnswer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		limit    int
		used     int
		reserved int
		children int
		want     int
	}{
		{name: "one child", limit: 12, used: 1, children: 1, want: 9},
		{name: "two children", limit: 12, used: 1, children: 2, want: 4},
		{name: "prior allocation", limit: 12, used: 2, reserved: 4, children: 2, want: 2},
		{name: "no verification room", limit: 4, used: 1, reserved: 1, children: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{Config: AgentConfig{MaxIterations: tc.limit}, Checkpoint: &Checkpoint{Iteration: tc.used, ReservedChildIterations: tc.reserved}}
			got, err := childDecisionBudget(task, tc.children)
			if tc.want == 0 {
				if err == nil || got != 0 {
					t.Fatalf("delegation left no parent verification room: budget=%d error=%v", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("child budget=%d error=%v, want %d", got, err, tc.want)
			}
		})
	}
}

func TestRestoreChildReservationsNeverReopensRootBudget(t *testing.T) {
	for _, tc := range []struct {
		name     string
		childOne int
		childTwo int
		reserved int
		want     int
	}{
		{name: "linked children", childOne: 4, childTwo: 5, want: 9},
		{name: "oversubscribed saved children", childOne: 6, childTwo: 6, want: 10},
		{name: "invalid saved child", childOne: 0, childTwo: 1, want: 10},
		{name: "larger existing reservation", childOne: 4, childTwo: 5, reserved: 10, want: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := &Task{ID: "parent", Config: AgentConfig{MaxIterations: 10}, Checkpoint: &Checkpoint{ReservedChildIterations: tc.reserved}, ChildHistory: []ChildLink{{ID: "one"}, {ID: "two"}}}
			children := []*Task{{ID: "one", Config: AgentConfig{MaxIterations: tc.childOne}}, {ID: "two", Config: AgentConfig{MaxIterations: tc.childTwo}}}
			changed := restoreChildReservations(append([]*Task{parent}, children...))
			if got := parent.Checkpoint.ReservedChildIterations; got != tc.want {
				t.Fatalf("reservation = %d, want %d", got, tc.want)
			}
			if changed[parent.ID] != (tc.reserved < tc.want) {
				t.Fatalf("unexpected changed flag: %v", changed)
			}
		})
	}
}
