package server

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// No timer or executor is owned here. The application's existing runner invokes
// this bounded callback and owns cancellation/drain of its preparation work.
type scheduleController struct {
	server    *Server
	clock     func() time.Time
	boot      time.Time
	epoch     string
	mu        sync.Mutex
	last      time.Time
	recovered bool
	skipNext  bool
}

func newScheduleController(s *Server, clock func() time.Time) *scheduleController {
	return &scheduleController{server: s, clock: clock, boot: clock().UTC(), epoch: "schedule-" + uuid.NewString()}
}

func (c *scheduleController) coordinate(ctx context.Context, _ time.Time) (result error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.server
	now := c.clock()
	if !c.last.IsZero() && c.last.Unix() == now.Unix() {
		return nil
	}
	forceMiss := c.skipNext || c.last.IsZero() && now.Unix()-c.boot.Unix() > 5 || !c.last.IsZero() && now.Unix()-c.last.Unix() > 5
	c.last = now
	c.skipNext = false
	defer func() {
		if result != nil {
			c.skipNext = true
		}
	}()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.agentRunner == nil || s.agentManager == nil {
		return agents.ErrRunStorage
	}
	if err := s.agentRunner.ReconcileScheduleLedger(ctx, s.workspaceID, !c.recovered); err != nil {
		return err
	}
	c.recovered = true
	// Revocation closes new dispatch immediately in providers. Also stop owned
	// queued/running roots here; existing settlement protections remain truthful.
	for _, task := range s.agentManager.ScheduledRootCandidates(s.workspaceID) {
		if task.Config.Automation == nil || task.ParentID != "" || task.Config.Profile == nil || task.Config.Profile.Workspace != s.workspaceID {
			continue
		}
		if task.Status == agents.TaskCompleted || task.Status == agents.TaskFailed || task.Status == agents.TaskCancelled {
			continue
		}
		if s.agentManager.ScheduleAllowsTask(task) != nil || !s.scheduleActorPermission(task.Actor, users.PermissionChat) {
			_, _ = s.agentRunner.Stop(task.ID, task.Actor, "cancel", "")
		}
	}
	due, err := s.agentManager.DueSchedules(s.workspaceID, now)
	if err != nil {
		return err
	}
	for _, schedule := range due {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fresh := schedule.ReviewEpoch == c.epoch
		late := forceMiss || schedule.NextAt.Before(c.boot) && !fresh || now.Sub(schedule.NextAt) >= time.Minute
		if late {
			if _, err = s.agentManager.ClaimPreparedScheduleOccurrence(schedule, schedule.Config, now, now.Add(time.Second)); err != nil {
				return err
			}
			continue
		}
		prepared := schedule.Config
		issue := s.validateScheduleResources(ctx, schedule.Actor, prepared, false)
		if issue == nil {
			issue = s.prepareScheduleKnowledge(ctx, schedule.Settings.Prompt, &prepared)
		}
		current := c.clock()
		occurrence, err := s.agentManager.ClaimPreparedScheduleOccurrence(schedule, prepared, current, c.boot, c.epoch)
		if err != nil {
			return err
		}
		if occurrence.State != "claimed" {
			continue
		}
		if issue != nil {
			if err = s.agentManager.BlockScheduleOccurrence(schedule, *occurrence, scheduleFailureCode(issue)); err != nil {
				return err
			}
			continue
		}
		if issue = s.validateScheduleResources(ctx, schedule.Actor, prepared, false); issue != nil {
			if err = s.agentManager.BlockScheduleOccurrence(schedule, *occurrence, scheduleFailureCode(issue)); err != nil {
				return err
			}
			continue
		}
		config, err := agents.PreparedScheduleOccurrenceConfig(schedule, *occurrence, prepared)
		if err != nil {
			return err
		}
		root, _, err := s.agentRunner.CreateRequest(schedule.Settings.Prompt, config.Profile.Settings.Model, schedule.Actor, config, occurrence.RequestID, occurrence.RequestDigest)
		if err != nil {
			_ = s.agentManager.BlockScheduleOccurrence(schedule, *occurrence, "root_admission_unavailable")
			return err
		}
		if err = s.agentManager.LinkScheduleOccurrence(schedule, *occurrence, root.ID); err != nil {
			return err
		}
		if err = s.agentManager.QueueScheduledRoot(root.ID, root.Actor); err != nil {
			return err
		}
		// Capacity pressure leaves the same pending root in the normal queue.
		// No new request is created and no executing call is retried here.
		runContext := s.runtimeCtx
		if runContext == nil {
			runContext = context.Background()
		}
		_, _ = s.agentRunner.Continue(runContext, root.ID, root.Actor, "start", "", true)
	}
	return nil
}
