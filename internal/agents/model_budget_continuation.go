package agents

import "fmt"

// A continued assignment owns the same unresolved correction sequence. Move
// its charge in the command transaction, never delete it merely on admission.
// The first valid decision can resolve it through normal checkpoint publication.
func transferModelCorrections(owner, previous, next *Task) error {
	b := owner.Checkpoint.ModelBudget
	if b == nil {
		return nil
	}
	if _, err := b.corrections(); err != nil {
		return err
	}
	count := b.Repairs[previous.ID]
	if count == 0 {
		return nil
	}
	if previous.ID == next.ID || b.Repairs[next.ID] != 0 || next.Checkpoint.DecisionRepairs != 0 || next.Checkpoint.ProposalRepairs != 0 {
		return fmt.Errorf("model correction continuation identity is inconsistent")
	}
	delete(b.Repairs, previous.ID)
	delete(b.AuxiliaryRepairs, previous.ID)
	b.Repairs[next.ID] = count
	// Carry only corrective debt, never an executable proposal, receipt or grant.
	next.Checkpoint.DecisionRepairs = count
	return nil
}
