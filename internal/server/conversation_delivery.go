package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// This controller owns no executor or goroutine. The existing runner invokes
// its bounded preparation callback and drains it during service shutdown.
type conversationDeliveryController struct {
	server         *Server
	mu             sync.Mutex
	last           time.Time
	cursor         int64
	decisionCursor int64
	stopCursor     int64
}

func (c *conversationDeliveryController) coordinate(ctx context.Context, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.last.IsZero() && now.Sub(c.last) < time.Second {
		return nil
	}
	c.last = now
	s := c.server
	if s.agentManager == nil || s.agentRunner == nil {
		return agents.ErrRunStorage
	}
	if err := c.settleStoppedInputs(ctx); err != nil {
		return err
	}
	if err := c.incorporate(ctx, now); err != nil {
		return err
	}
	page, err := s.agentManager.ConversationDeliveries(s.workspaceID, c.cursor, 8)
	if err != nil {
		return err
	}
	for _, candidate := range page.Items {
		if err = ctx.Err(); err != nil {
			return err
		}
		input := candidate.Input
		if input.DeliveryIssue != nil && now.Before(input.DeliveryIssue.RetryAt) {
			continue
		}
		_, err = s.prepareConversationTurn(ctx, candidate.Scope, input.ConversationID, input.ID)
		if err == nil || errors.Is(err, agents.ErrRunConflict) {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, agents.ErrRunStorage) {
			return err
		}
		issue := api.ConversationDeliveryIssue{Code: "conversation_preparation_unavailable", Message: "Input is saved; its model or authorized context is not ready for delivery.", RetryAt: now.Add(30 * time.Second).UTC()}
		var service *serviceError
		if errors.As(err, &service) {
			issue.Message = service.message
			if service.code != "" {
				issue.Code = service.code
			}
		}
		if err = s.agentManager.DeferConversationDelivery(candidate.Scope, input.ConversationID, input.ID, issue); err != nil && !errors.Is(err, agents.ErrRunConflict) {
			return err
		}
	}
	c.cursor = page.NextCursor
	if !page.HasMore {
		c.cursor = 0
	}
	return nil
}

func (s *Server) coordinateServiceWork(ctx context.Context, now time.Time) error {
	return errors.Join(s.conversationDelivery.coordinate(ctx, now), s.schedules.coordinate(ctx, now))
}
