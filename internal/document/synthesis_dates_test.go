package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestSynthesisNumericDatesMustMatchWholeSourceTuple(t *testing.T) {
	for _, tc := range []struct {
		claim, evidence string
		want            bool
	}{
		{"7月31日前提交，农村诊所可在8月15日前提交，费用为25美元。", "Submit by 31 July. Rural clinics: 15 August. Fee USD 25.", true},
		{"7月30日までに提出。", "Submit by 31 July.", false},
		{"8월 31일까지 제출.", "31 July. Exception: 15 August.", false},
		{"2026年7月31日", "July 31, 2026", true},
		{"2027年7月31日", "July 31, 2026", false},
		{"2026年7月31日", "31 July", false},
		{"2026-07-31", "31 July 2026", true},
		{"7월 31일", "2026-07-31", true},
		{"2024년2월29일", "29 February 2024", true},
		{"2025年2月29日", "29 February 2025", false},
		{"1237月31日", "31 July", false},
		{"12月31日", "112月31日", false},
		{"7月31日。Fee 7.", "31 July. Fee 25.", false},
		{"31/07", "31 July", false},
		{"Fee 31.25", "31 July. Fee 25.", false},
	} {
		if got := synthesisNumbersSupported(tc.claim, tc.evidence); got != tc.want {
			t.Errorf("%q / %q: %v want %v", tc.claim, tc.evidence, got, tc.want)
		}
	}
}

func TestSynthesisCalendarValidationPreservesHistoricalReceipts(t *testing.T) {
	tr, batch := synthesisFixture(t, "requirements")
	draft := synthesisDraft(batch)
	draft.Items[0].Text = "6月30日前提交；农村办事处可于7月31日前提交。"
	raw, _ := json.Marshal(draft)
	hash := sha256.Sum256(raw)
	identity := synthesisIdentity()
	modelID, _ := identity.ID()
	legacy, err := tr.compileCandidates(batch, raw, false)
	if err != nil || len(legacy.Candidates) != 0 {
		t.Fatal(legacy, err)
	}
	receipt := &SynthesisBatchReceipt{Method: "model", BatchID: batch.ID, ModelIdentity: modelID, UnitOrdinal: batch.UnitOrdinal, Primary: batch.Primary, EndOfUnit: batch.EndOfUnit, PromptTokens: 1000, ResponseSHA256: hex.EncodeToString(hash[:]), Result: legacy}
	if err = tr.ValidateReceipt(batch, identity, receipt, raw); err != nil {
		t.Fatal("historical receipt changed", err)
	}
	current, err := tr.CompileCandidates(batch, raw)
	if err != nil || len(current.Candidates) != 1 {
		t.Fatal(current, err)
	}
	receipt.Result = current
	if err = tr.ValidateReceipt(batch, identity, receipt, raw); err == nil {
		t.Fatal("old validation version silently upgraded")
	}
	receipt.Validation = "source-values-v2"
	if err = tr.ValidateReceipt(batch, identity, receipt, raw); err != nil {
		t.Fatal("current receipt rejected", err)
	}
	receipt.Validation = "future-unknown"
	if err = tr.ValidateReceipt(batch, identity, receipt, raw); err == nil {
		t.Fatal("unknown validation policy accepted")
	}
}
