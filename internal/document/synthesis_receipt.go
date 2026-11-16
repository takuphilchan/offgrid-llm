package document

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
)

func (t *SynthesisTraversal) PlanID() string { return t.plan.ID }

// Empty native spans require no model call. Extraction gaps (such as an unread
// figure) remain attached; empty extracted text never implies an empty original.
func (t *SynthesisTraversal) EmptyReceipt(batch *SynthesisBatch, identity SynthesisModelIdentity) (*SynthesisBatchReceipt, error) {
	if err := t.VerifyBatch(batch); err != nil {
		return nil, err
	}
	if strings.TrimSpace(batch.Evidence[0].Text) != "" {
		return nil, errors.New("source batch is not empty")
	}
	modelID, err := identity.ID()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(nil)
	return &SynthesisBatchReceipt{Method: "empty_source", BatchID: batch.ID, ModelIdentity: modelID, UnitOrdinal: batch.UnitOrdinal, Primary: batch.Primary, EndOfUnit: batch.EndOfUnit, ResponseSHA256: hex.EncodeToString(hash[:]), Result: &SynthesisCandidates{BatchID: batch.ID, Candidates: []SynthesisCandidate{}, Gaps: append([]AnalysisGap{}, batch.Gaps...)}}, nil
}

// Check persisted receipts against the private retained model response and
// exact source batch. Raw responses must remain inside the job's ACL boundary.
func (t *SynthesisTraversal) ValidateReceipt(batch *SynthesisBatch, identity SynthesisModelIdentity, receipt *SynthesisBatchReceipt, raw []byte) error {
	if receipt == nil {
		return errors.New("missing synthesis receipt")
	}
	if receipt.Method == "empty_source" {
		expected, err := t.EmptyReceipt(batch, identity)
		if err != nil || len(raw) != 0 || !reflect.DeepEqual(expected, receipt) {
			return errors.New("invalid empty synthesis receipt")
		}
		return nil
	}
	if receipt.Method != "model" || (receipt.Validation != "" && receipt.Validation != "source-values-v2") || !MeasuredSynthesisFit(receipt.PromptTokens, identity.ContextTokens, identity.ResponseTokens) {
		return errors.New("invalid synthesis generation receipt")
	}
	modelID, err := identity.ID()
	if err != nil {
		return err
	}
	result, err := t.compileCandidates(batch, raw, receipt.Validation == "source-values-v2")
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	expected := &SynthesisBatchReceipt{Validation: receipt.Validation, Method: "model", BatchID: batch.ID, ModelIdentity: modelID, UnitOrdinal: batch.UnitOrdinal, Primary: batch.Primary, EndOfUnit: batch.EndOfUnit, PromptTokens: receipt.PromptTokens, ResponseSHA256: hex.EncodeToString(hash[:]), Result: result}
	if !reflect.DeepEqual(expected, receipt) {
		return errors.New("synthesis receipt does not match retained response")
	}
	return nil
}
