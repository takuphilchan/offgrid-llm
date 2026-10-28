package agents

import (
	"context"
	"time"
)

type unsettledOccurrence struct {
	ID, Actor, State, JobID, RequestID, Digest string
}

func terminalScheduleTask(task *Task) bool {
	if task == nil || task.Checkpoint != nil && task.Checkpoint.ExecutingCall != "" || HasOwnedExecutionProcesses(task) {
		return false
	}
	return task.Status == TaskCompleted || task.Status == TaskFailed || task.Status == TaskCancelled
}

// ReconcileScheduleLedger observes task state only. It never calls Continue or
// creates a replacement root. bootRecovery is used once by the owned coordinator
// after exclusive workspace recovery, not by ordinary refresh/history endpoints.
func (r *Runner) ReconcileScheduleLedger(ctx context.Context, workspace string, bootRecovery bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	m := r.manager
	db, err := m.scheduleDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `SELECT o.id,s.actor,o.state,o.job_id,o.request_id,o.request_digest FROM agent_schedule_occurrences o JOIN agent_schedules s ON o.schedule_id=s.id WHERE s.workspace=? AND o.state IN ('claimed','admitted','interrupted') ORDER BY o.created_at,o.id LIMIT 256`, workspace)
	if err != nil {
		return err
	}
	items := []unsettledOccurrence{}
	for rows.Next() {
		var item unsettledOccurrence
		if err = rows.Scan(&item.ID, &item.Actor, &item.State, &item.JobID, &item.RequestID, &item.Digest); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if item.State == "claimed" && !bootRecovery {
			continue
		}
		jobID := item.JobID
		if jobID == "" && bootRecovery {
			for _, candidate := range m.tasks {
				if candidate.Actor == item.Actor && candidate.RequestID == item.RequestID && candidate.RequestDigest == item.Digest {
					jobID = candidate.ID
					break
				}
			}
		}
		state, reason := item.State, ""
		if jobID == "" {
			if !bootRecovery {
				continue
			}
			state, reason = "missed", "restart_before_admission"
		} else {
			task := m.tasks[jobID]
			if task == nil || task.Actor != item.Actor || task.RequestID != item.RequestID || task.RequestDigest != item.Digest {
				state, reason = "interrupted", "saved_root_unavailable"
			} else {
				settled := terminalScheduleTask(task)
				if _, active := r.active[jobID]; active {
					settled = false
				}
				for _, child := range m.tasks {
					if child.ParentID == jobID {
						_, active := r.active[child.ID]
						if active || !terminalScheduleTask(child) {
							settled = false
						}
					}
				}
				if settled {
					state = string(task.Status)
				} else if bootRecovery || task.Status == TaskInterrupted || task.Status == TaskUncertain {
					state, reason = "interrupted", "review_saved_root"
				}
			}
		}
		if state == item.State && jobID == item.JobID {
			continue
		}
		result, err := db.ExecContext(ctx, `UPDATE agent_schedule_occurrences SET state=?,reason=?,job_id=? WHERE id=? AND state=? AND job_id=?`, state, reason, jobID, item.ID, item.State, item.JobID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrContextConflict
		}
	}
	return nil
}

// ScheduleOccurrenceConfig returns the immutable request material for an
// already-claimed slot. It is not a public operation or permission to dispatch.
func ScheduleOccurrenceConfig(schedule *AgentSchedule, o ScheduleOccurrence) (AgentConfig, error) {
	if schedule == nil {
		return AgentConfig{}, ErrSchedule
	}
	return PreparedScheduleOccurrenceConfig(schedule, o, schedule.Config)
}

func PreparedScheduleOccurrenceConfig(schedule *AgentSchedule, o ScheduleOccurrence, prepared AgentConfig) (AgentConfig, error) {
	if schedule == nil || !validOccurrence(&o) || o.Schedule != schedule.ID || o.Revision != schedule.Revision {
		return AgentConfig{}, ErrSchedule
	}
	if !sameScheduleAuthority(schedule.Config, prepared) {
		return AgentConfig{}, ErrSchedule
	}
	config, digest := preparedOccurrenceRequest(schedule, o, prepared)
	if digest != o.RequestDigest {
		return AgentConfig{}, ErrContextConflict
	}
	return config, nil
}

func sameScheduleOrigin(a, b *ScheduleOrigin) bool {
	return a != nil && b != nil && a.Version == b.Version && a.Schedule == b.Schedule && a.Revision == b.Revision && a.Occurrence == b.Occurrence && a.DueAt.Equal(b.DueAt)
}

// validateAutomationPin is structural only. Live ownership/policy/occurrence
// linkage is rechecked by service admission and each scheduled dispatch.
func validateAutomationPin(task *Task) bool {
	if task.Config.Automation == nil {
		return true
	}
	pin := task.Config.Automation
	return pin.Version == 1 && pin.Revision > 0 && pin.Schedule != "" && pin.Occurrence != "" && !pin.DueAt.IsZero() && pin.DueAt.Nanosecond() == 0 && pin.DueAt.Second() == 0 && task.ParentID == "" && task.Config.Profile != nil && task.Config.Profile.Actor == task.Actor && task.Config.TaskFirst && task.Config.RestrictComputer && task.Config.ComputerSession == "" && !task.Config.ComputerAccessOnDemand && task.Config.RequestedComputerScope == "" && task.Config.TimeoutPerStep > time.Duration(0)
}
