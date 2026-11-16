package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
)

// Model identity is pinned when a durable job is accepted. A changed model,
// runtime, language, context or protocol requires a new run, not mixed results.
type SynthesisModelIdentity struct {
	ModelID        string          `json:"model_id"`
	ModelSHA256    string          `json:"model_sha256"`
	Runtime        RuntimeIdentity `json:"runtime"`
	Language       string          `json:"language"`
	ContextTokens  int             `json:"context_tokens"`
	ResponseTokens int             `json:"response_tokens"`
	Protocol       string          `json:"protocol"`
}

func (i SynthesisModelIdentity) Validate() error {
	hash, err := hex.DecodeString(i.ModelSHA256)
	if err != nil || len(hash) != 32 || i.ModelID == "" || len(i.ModelID) > 256 || i.Runtime.Name == "" || len(i.Runtime.Name) > 128 || i.Runtime.Revision == "" || len(i.Runtime.Revision) > 256 || !synthesisLanguage.MatchString(i.Language) || i.Protocol != SynthesisProtocol || !MeasuredSynthesisFit(1, i.ContextTokens, i.ResponseTokens) {
		return errors.New("invalid synthesis model identity")
	}
	if i.Runtime.SHA256 != "" {
		hash, err = hex.DecodeString(i.Runtime.SHA256)
		if err != nil || len(hash) != 32 {
			return errors.New("invalid synthesis runtime digest")
		}
	}
	return nil
}
func (i SynthesisModelIdentity) ID() (string, error) {
	if err := i.Validate(); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(i)
	hash := sha256.Sum256(raw)
	return "synthesis-model-" + hex.EncodeToString(hash[:]), nil
}

// Implementations must resolve an installed local model, enforce the pinned
// package/runtime identity, and hold one generation lease across count and call.
// No tools or external providers are part of this protocol. Durable workers
// release this lease between batches so foreground Chat can be admitted.
type SynthesisLocalProvider interface {
	WithModel(context.Context, SynthesisModelIdentity, func(SynthesisLocalSession) error) error
}
type SynthesisLocalSession interface {
	Identity() SynthesisModelIdentity
	Count(context.Context, SynthesisPrompt) (int, error)
	Generate(context.Context, SynthesisPrompt, int) (raw []byte, finishReason string, err error)
}
type SynthesisBatchReceipt struct {
	Validation     string               `json:"validation,omitempty"`
	Method         string               `json:"method"`
	BatchID        string               `json:"batch_id"`
	ModelIdentity  string               `json:"model_identity"`
	UnitOrdinal    int                  `json:"unit_ordinal"`
	Primary        Span                 `json:"primary"`
	EndOfUnit      bool                 `json:"end_of_unit"`
	PromptTokens   int                  `json:"prompt_tokens"`
	ResponseSHA256 string               `json:"response_sha256"`
	Result         *SynthesisCandidates `json:"result"`
}

// ProcessSynthesisBatch performs no publication. The durable owner must recheck
// access and commit receipt/progress atomically, then apply support assessment.
// guard is service-owned current-access checking, not a model-supplied callback.
func (t *SynthesisTraversal) ProcessSynthesisBatch(ctx context.Context, batch *SynthesisBatch, identity SynthesisModelIdentity, provider SynthesisLocalProvider, guard func() error) (*SynthesisBatchReceipt, error) {
	execution, err := t.ExecuteSynthesisBatch(ctx, batch, identity, provider, guard)
	if err != nil {
		return nil, err
	}
	return execution.Receipt, nil
}

// RawResponse is retained only in private job storage for checkpoint validation.
// It is deliberately excluded from API serialization and ordinary logs.
type SynthesisExecution struct {
	Receipt     *SynthesisBatchReceipt `json:"receipt"`
	RawResponse []byte                 `json:"-"`
}

func (t *SynthesisTraversal) ExecuteSynthesisBatch(ctx context.Context, batch *SynthesisBatch, identity SynthesisModelIdentity, provider SynthesisLocalProvider, guard func() error) (*SynthesisExecution, error) {
	if provider == nil || guard == nil {
		return nil, errors.New("synthesis runtime or access guard unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	modelID, err := identity.ID()
	if err != nil {
		return nil, err
	}
	prompt, err := t.Prompt(batch, identity.Language)
	if err != nil {
		return nil, err
	}
	if err = guard(); err != nil {
		return nil, err
	}
	var receipt *SynthesisBatchReceipt
	var retained []byte
	calls := 0
	err = provider.WithModel(ctx, identity, func(session SynthesisLocalSession) error {
		calls++
		if calls != 1 || session == nil || !reflect.DeepEqual(session.Identity(), identity) {
			return errors.New("synthesis runtime identity changed")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		count, err := session.Count(ctx, prompt)
		if err != nil {
			return err
		}
		if !MeasuredSynthesisFit(count, identity.ContextTokens, identity.ResponseTokens) {
			return errors.New("synthesis prompt exceeds measured context")
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = guard(); err != nil {
			return err
		}
		if !reflect.DeepEqual(session.Identity(), identity) {
			return errors.New("synthesis runtime changed after token count")
		}
		raw, finish, err := session.Generate(ctx, prompt, identity.ResponseTokens)
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = guard(); err != nil {
			return err
		}
		if finish != "stop" {
			return errors.New("synthesis response was not completed")
		}
		if !reflect.DeepEqual(session.Identity(), identity) {
			return errors.New("synthesis runtime changed during generation")
		}
		result, err := t.CompileCandidates(batch, raw)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		retained = append([]byte(nil), raw...)
		receipt = &SynthesisBatchReceipt{Validation: "source-values-v2", Method: "model", BatchID: batch.ID, ModelIdentity: modelID, UnitOrdinal: batch.UnitOrdinal, Primary: batch.Primary, EndOfUnit: batch.EndOfUnit, PromptTokens: count, ResponseSHA256: hex.EncodeToString(digest[:]), Result: result}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if calls != 1 || receipt == nil {
		return nil, errors.New("synthesis provider produced no receipt")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = guard(); err != nil {
		return nil, err
	}
	return &SynthesisExecution{Receipt: receipt, RawResponse: retained}, nil
}
