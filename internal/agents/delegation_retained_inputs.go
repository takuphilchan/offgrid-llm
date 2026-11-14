package agents

import "fmt"

// A new test/research assignment can depend on an exact completed assignment in
// this attempt. Keep that link in the graph as settled evidence; do not create
// another worker or reserve another model allocation for it.
func (r *Runner) retainedDelegationDependencies(root *Task, specs []ChildSpec) ([]ChildLink, error) {
	keys := map[string]bool{}
	for _, spec := range specs {
		keys[spec.Key] = true
	}
	retained := []ChildLink{}
	all := append([]ChildSpec(nil), specs...)
	for index := 0; index < len(all); index++ {
		for _, key := range all[index].DependsOn {
			if keys[key] {
				continue
			}
			if root.WorkID == "" {
				return nil, fmt.Errorf("Unknown dependency %s", key)
			}
			var found *ChildLink
			for _, link := range root.ChildHistory {
				if link.Invalidated || link.Spec.Key != key {
					continue
				}
				if found != nil {
					return nil, fmt.Errorf("Dependency %s is ambiguous; use a unique assignment key", key)
				}
				copy := link
				found = &copy
			}
			if found == nil {
				return nil, fmt.Errorf("Unknown dependency %s", key)
			}
			child, ok := r.manager.GetTask(found.ID)
			if !ok || child.ParentID != root.ID || child.Actor != root.Actor || child.InstructionVersion != root.InstructionVersion || !workerDependencyComplete(root, child) {
				return nil, fmt.Errorf("Dependency %s has no current completed result", key)
			}
			keys[key] = true
			retained = append(retained, *found)
			all = append(all, found.Spec)
			if len(all) > childHistoryLimit(root) {
				return nil, fmt.Errorf("Dependency graph exceeds retained work limits")
			}
		}
	}
	if err := validateChildSpecs(all); err != nil {
		return nil, err
	}
	return retained, nil
}
