package server

import (
	"context"
	"errors"
	"reflect"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// InstallQualifiedAgentTools is a startup-only composition hook. It is not an
// HTTP/model option and never edits saved jobs or manufactures qualification.
func (s *Server) InstallQualifiedAgentTools(target, live inference.AgentToolTarget) error {
	if s.config == nil || s.agentRunner != nil || s.agentToolPolicy != nil {
		return errors.New("agent tool policy must be installed before binding the runner")
	}
	observed, err := s.observeAgentToolTarget(live)
	if err != nil {
		return err
	}
	policy, err := inference.NewAgentToolPolicy(models.NewCapabilityEvidenceStore(s.config.DataDir), target, observed)
	if err != nil {
		return err
	}
	s.agentToolPolicy = policy
	return nil
}

// Bind installation-owned artifact/hardware measurements to the actual child
// serving this model. The callback still owns file and hardware verification;
// /props cannot establish those identities or manufacture workflow evidence.
func (s *Server) observeAgentToolTarget(live inference.AgentToolTarget) (inference.AgentToolTarget, error) {
	owner, ok := s.llamaProcess.(interface {
		ObserveToolRuntime(context.Context, string, int) (*inference.ToolRuntimeObservation, error)
	})
	if !ok || live == nil {
		return nil, errors.New("native agent tools require owned runtime observation and artifact measurements")
	}
	return func(ctx context.Context, model string, window int) (*models.CapabilityEvidenceTarget, error) {
		before, err := owner.ObserveToolRuntime(ctx, model, window)
		if err != nil {
			return nil, err
		}
		target, err := live(ctx, model, window)
		if err != nil || target == nil {
			return nil, err
		}
		after, err := owner.ObserveToolRuntime(ctx, model, window)
		if err != nil {
			return nil, err
		}
		if before == nil || after == nil || before.ModelID != model || before.Generation == 0 || !reflect.DeepEqual(before, after) {
			return nil, errors.New("native agent runtime changed during measurement")
		}
		info, fingerprint := after.Info, target.Fingerprint
		if info.Context != window || fingerprint.ContextWindow != info.Context || fingerprint.RuntimeBuild != info.Build || fingerprint.TemplateSHA256 != info.TemplateSHA256 || info.Build == "" || info.TemplateSHA256 == "" || after.ConfigSHA256 == "" || fingerprint.RuntimeConfigSHA256 != after.ConfigSHA256 {
			return nil, errors.New("native agent measurement does not match the owned runtime")
		}
		if info.SupportsTools != nil && !*info.SupportsTools || info.SupportsToolCalls != nil && !*info.SupportsToolCalls {
			return nil, errors.New("owned runtime reports native tool calling unavailable")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return target, nil
	}, nil
}

// Called with a model lease held, before either token counting or generation.
// A stale native pin must stop; silently returning to constrained decoding would
// change a saved job's execution contract after earlier effects.
func (s *Server) configureAgentRequest(ctx context.Context, request *api.ChatCompletionRequest, task *agents.Task) error {
	configureComputerRequest(request, task)
	if task.Config.ToolAdapter == nil {
		return nil
	}
	if err := agents.ValidateAgentToolScope(task.Config.ToolAdapter); err != nil {
		return err
	}
	if !task.Config.TaskFirst || task.Config.TextOnlyInference {
		return errors.New("native tool adapter is not valid for this job")
	}
	if err := s.agentToolPolicy.Validate(ctx, task.Model, s.taskContextWindow(task), task.Config.ToolAdapter); err != nil {
		return err
	}
	request.SingleStepTools = false
	return nil
}
