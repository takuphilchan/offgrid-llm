package server

import (
	"context"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (c *conversationDeliveryController) incorporate(ctx context.Context, now time.Time) error {
	s := c.server
	page, err := s.agentManager.ConversationDecisions(s.workspaceID, c.decisionCursor, 8)
	if err != nil {
		return err
	}
	for _, candidate := range page.Items {
		if err = ctx.Err(); err != nil {
			return err
		}
		input := candidate.Input
		if input.DecisionIssue != nil && now.Before(input.DecisionIssue.RetryAt) {
			continue
		}
		if input.Decision.Kind == "new_work" {
			_, err = s.prepareConversationInputWork(ctx, candidate.Scope, input.ConversationID, input.ID)
		} else {
			_, err = s.agentRunner.ApplyConversationDecision(candidate.Scope, input.ConversationID, input.ID)
		}
		if err == nil {
			continue
		}
		issue := api.ConversationDeliveryIssue{Code: "conversation_decision_waiting", Message: "The recorded decision is waiting for earlier input or dispatched operations to settle. No change has been applied.", RetryAt: now.Add(time.Second).UTC()}
		var service *serviceError
		if errors.As(err, &service) {
			issue.Code, issue.Message = service.code, service.message
			issue.RetryAt = now.Add(30 * time.Second).UTC()
		} else if errors.Is(err, agents.ErrSourceAccess) {
			issue.Code, issue.Message = "conversation_decision_access_changed", "The decision remains saved but cannot be applied with the current actor or source access."
			issue.RetryAt = now.Add(30 * time.Second).UTC()
		} else if !errors.Is(err, agents.ErrRunConflict) {
			return err
		} else if input.Decision.Kind == "correction" {
			work, readErr := s.agentManager.WorkItem(candidate.Scope, input.WorkID)
			if readErr != nil {
				return readErr
			}
			version := input.Decision.InstructionVersion
			if input.RevalidatedAt != nil {
				version = input.RevalidatedInstructionVersion
			}
			if work.InstructionVersion != version {
				issue.Code, issue.Message = "conversation_decision_stale", "Work instructions changed after this correction was prepared. The saved correction needs revalidation before it can be applied."
				issue.RetryAt = now.Add(30 * time.Second).UTC()
			} else if work.Status == agents.TaskCancelled || work.Status == agents.TaskCompleted || work.Status == agents.TaskFailed {
				issue.Code, issue.Message = "conversation_decision_work_finished", "The focused work has already finished or stopped. This correction remains saved and has not restarted that work."
				issue.RetryAt = now.Add(30 * time.Second).UTC()
			}
		}
		if err = s.agentManager.DeferConversationDecision(candidate.Scope, input.ConversationID, input.ID, issue); err != nil && !errors.Is(err, agents.ErrRunConflict) {
			return err
		}
	}
	c.decisionCursor = page.NextCursor
	if !page.HasMore {
		c.decisionCursor = 0
	}
	return nil
}

func (c *conversationDeliveryController) settleStoppedInputs(ctx context.Context) error {
	page, err := c.server.agentManager.StoppingConversationInputs(c.server.workspaceID, c.stopCursor, 8)
	if err != nil {
		return err
	}
	for _, item := range page.Items {
		if err = ctx.Err(); err != nil {
			return err
		}
		if _, err = c.server.agentRunner.SettleStoppedConversationInput(item.Scope, item.Input.ConversationID, item.Input.ID); err != nil && !errors.Is(err, agents.ErrRunConflict) {
			return err
		}
	}
	c.stopCursor = page.NextCursor
	if !page.HasMore {
		c.stopCursor = 0
	}
	return nil
}
