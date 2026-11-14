package agents

import (
	"context"
	"testing"
)

func TestRetainedDelegationDependenciesUseCurrentSettledResults(t *testing.T) {
	for _, mode := range []string{"valid", "stale", "unfinished", "ambiguous", "invalidated", "cycle", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			m, r, scope, root, child := workerFixture(t)
			defer r.Shutdown(context.Background())
			if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
				t.Fatal(err)
			}
			root, _ = m.GetTask(root.ID)
			key := root.ChildHistory[0].Spec.Key
			switch mode {
			case "stale":
				root.InstructionVersion++
			case "unfinished":
				if _, err := m.updateTask(child.ID, func(task *Task) error { task.Status = TaskPending; return nil }); err != nil {
					t.Fatal(err)
				}
			case "ambiguous":
				root.ChildHistory = append(root.ChildHistory, root.ChildHistory[0])
			case "invalidated":
				root.ChildHistory[0].Invalidated = true
			case "cycle":
				root.ChildHistory[0].Spec.DependsOn = []string{"new-check"}
			case "legacy":
				root.WorkID = ""
			}
			before := len(m.ListTasks())
			links, err := r.retainedDelegationDependencies(root, []ChildSpec{{Key: "new-check", Goal: "Check the prior result", DependsOn: []string{key}}})
			if mode == "valid" {
				if err != nil || len(links) != 1 || links[0].ID != child.ID || len(m.ListTasks()) != before {
					t.Fatal("retained dependency recreated worker", links, err)
				}
			} else if err == nil {
				t.Fatal("invalid historical dependency accepted")
			}
		})
	}
}
