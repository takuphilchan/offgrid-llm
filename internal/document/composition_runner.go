package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
)

// Execute prepares one checkpoint, never publishes it. The durable caller must
// enforce current source access and atomically commit before calling Apply.
func (c *Composition) Execute(ctx context.Context, task *CompositionTask, identity SynthesisModelIdentity, provider SynthesisLocalProvider, guard func() error) (*CompositionExecution, error) {
	if task == nil || !reflect.DeepEqual(task, c.Next()) || provider == nil || guard == nil {
		return nil, errors.New("composition runtime or next task unavailable")
	}
	model, err := identity.ID()
	if err != nil {
		return nil, err
	}
	for _, previous := range c.receipts {
		if previous.ModelIdentity != model {
			return nil, errors.New("composition model identity changed")
		}
		break
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = guard(); err != nil {
		return nil, err
	}
	_, order, err := c.inputs(task)
	if err != nil {
		return nil, err
	}
	if len(order) == 0 {
		receipt := c.emptyReceipt(task, model)
		if err = guard(); err != nil {
			return nil, err
		}
		return &CompositionExecution{Receipt: receipt}, nil
	}
	prompt, err := c.Prompt(task, identity.Language)
	if err != nil {
		return nil, err
	}
	var execution *CompositionExecution
	calls := 0
	err = provider.WithModel(ctx, identity, func(session SynthesisLocalSession) error {
		calls++
		if calls != 1 || session == nil || !reflect.DeepEqual(session.Identity(), identity) {
			return errors.New("composition runtime identity changed")
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
			return errors.New("composition prompt exceeds measured context")
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = guard(); err != nil {
			return err
		}
		if !reflect.DeepEqual(session.Identity(), identity) {
			return errors.New("composition runtime changed after token count")
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
		if finish != "stop" || !reflect.DeepEqual(session.Identity(), identity) {
			return errors.New("composition generation incomplete or runtime changed")
		}
		result, err := c.Compile(task, raw)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		execution = &CompositionExecution{Receipt: &CompositionReceipt{Method: "model", Protocol: CompositionProtocol, PlanID: c.plan.ID, TaskID: task.ID, Ordinal: task.Ordinal, ModelIdentity: model, PromptTokens: count, ResponseSHA256: hex.EncodeToString(hash[:]), Result: result}, RawResponse: append([]byte{}, raw...)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if calls != 1 || execution == nil {
		return nil, errors.New("composition runtime produced no receipt")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = guard(); err != nil {
		return nil, err
	}
	return execution, nil
}

func (c *Composition) emptyReceipt(task *CompositionTask, model string) *CompositionReceipt {
	hash := sha256.Sum256(nil)
	return &CompositionReceipt{Method: "no_supported_inputs", Protocol: CompositionProtocol, PlanID: c.plan.ID, TaskID: task.ID, Ordinal: task.Ordinal, ModelIdentity: model, ResponseSHA256: hex.EncodeToString(hash[:]), Result: &CompositionResult{Sections: []CompositionSection{}, Gaps: []CompositionGap{{task.ID, "composition_no_supported_inputs"}}, SemanticReviewRequired: true}}
}

// Apply verifies a retained checkpoint before advancing the in-memory cursor.
// Reconstructing the same tree on restart never invokes a model or silently
// upgrades an older validation protocol.
func (c *Composition) Apply(identity SynthesisModelIdentity, receipt *CompositionReceipt, raw []byte) error {
	task := c.Next()
	if task == nil || receipt == nil {
		return errors.New("unexpected composition checkpoint")
	}
	model, err := identity.ID()
	if err != nil {
		return err
	}
	for _, previous := range c.receipts {
		if previous.ModelIdentity != model {
			return errors.New("composition model identity changed")
		}
		break
	}
	_, order, err := c.inputs(task)
	if err != nil {
		return err
	}
	var expected *CompositionReceipt
	if len(order) == 0 {
		if len(raw) != 0 {
			return errors.New("empty composition has raw model output")
		}
		expected = c.emptyReceipt(task, model)
	} else {
		if !MeasuredSynthesisFit(receipt.PromptTokens, identity.ContextTokens, identity.ResponseTokens) {
			return errors.New("invalid composition token count")
		}
		result, err := c.Compile(task, raw)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		expected = &CompositionReceipt{Method: "model", Protocol: CompositionProtocol, PlanID: c.plan.ID, TaskID: task.ID, Ordinal: task.Ordinal, ModelIdentity: model, PromptTokens: receipt.PromptTokens, ResponseSHA256: hex.EncodeToString(hash[:]), Result: result}
	}
	if !reflect.DeepEqual(expected, receipt) {
		return errors.New("composition receipt differs from retained response")
	}
	// Deep-copy so caller mutation cannot rewrite validated ancestry.
	data, _ := json.Marshal(expected)
	var stored CompositionReceipt
	_ = json.Unmarshal(data, &stored)
	c.receipts[task.ID] = &stored
	return nil
}

func (c *Composition) PlanID() string { return c.plan.ID }

// Validate without moving the worker cursor before its transaction commits.
func (c *Composition) ValidateReceipt(identity SynthesisModelIdentity, receipt *CompositionReceipt, raw []byte) error {
	copy := *c
	copy.receipts = make(map[string]*CompositionReceipt, len(c.receipts))
	for key, value := range c.receipts {
		copy.receipts[key] = value
	}
	return copy.Apply(identity, receipt, raw)
}
