package computer

import "testing"

func TestApprovalModesNeverOverrideForbiddenActions(t *testing.T) {
	for _, mode := range []ApprovalMode{ApprovalAskEveryTime, ApprovalScopedChanges, ApprovalFullTask} {
		if mode.Allows(ActionForbidden) {
			t.Fatalf("%s allowed a forbidden action", mode)
		}
	}
	if ApprovalAskEveryTime.Allows(ActionReversible) {
		t.Fatal("ask mode bypassed exact approval")
	}
	if !ApprovalScopedChanges.Allows(ActionReversible) || ApprovalScopedChanges.Allows(ActionConsequential) {
		t.Fatal("scoped policy boundary changed")
	}
	if !ApprovalFullTask.Allows(ActionReversible) || !ApprovalFullTask.Allows(ActionConsequential) {
		t.Fatal("full task did not allow typed in-scope work")
	}
}

func TestOperationClassificationUsesHostControlContext(t *testing.T) {
	if got := ClassifyOperation(Operation{Kind: "activate", Element: "control"}, "Save draft", "button"); got != ActionReversible {
		t.Fatalf("save draft: %s", got)
	}
	if got := ClassifyOperation(Operation{Kind: "activate", Element: "control"}, "Submit report", "button"); got != ActionConsequential {
		t.Fatalf("submission: %s", got)
	}
	if got := ClassifyOperation(Operation{Kind: "activate", Element: "control"}, "Pay now", "button"); got != ActionForbidden {
		t.Fatalf("payment: %s", got)
	}
	if got := ClassifyOperation(Operation{Kind: "shortcut", Element: "control", Shortcut: "save"}, "Document", "editor"); got != ActionReversible {
		t.Fatalf("save shortcut: %s", got)
	}
}
