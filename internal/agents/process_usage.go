package agents

import (
	"encoding/json"
	"math"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func recordProcessReceipt(task *Task, request execution.Request, receipt execution.Receipt) error {
	b := task.Checkpoint.ProcessBudget
	if b == nil {
		return nil
	} // Older records have no measured accounting.
	key := request.OperationID
	reservation, exists := b.Reservations[key]
	if request.Operation != "process_start" {
		var args struct {
			Handle string `json:"handle"`
		}
		if json.Unmarshal(request.Arguments, &args) != nil || args.Handle == "" {
			return ErrRunConflict
		}
		exists = false
		for id, saved := range b.Reservations {
			if saved.RunID == task.ID && saved.Reference == *task.Config.Execution && saved.Handle == args.Handle {
				key, reservation, exists = id, saved, true
				break
			}
		}
	}
	if !exists {
		return nil
	} // Legacy starts retain their conservative capacity.
	if reservation.RunID != task.ID || reservation.Reference != *task.Config.Execution {
		return ErrRunConflict
	}
	if receipt.State == "rejected" && request.Operation == "process_start" {
		if reservation.Handle != "" {
			return ErrRunConflict
		}
		return settleProcessReservation(b, key, reservation, 0, true)
	}
	if receipt.State != "complete" && receipt.State != "dispatched" {
		return nil
	}
	var process execution.ProcessResult
	if json.Unmarshal(receipt.Result, &process) != nil || process.Handle == "" {
		return ErrRunConflict
	}
	if reservation.Handle != "" && reservation.Handle != process.Handle {
		return ErrRunConflict
	}
	for id, saved := range b.Reservations {
		if id != key && saved.RunID == task.ID && saved.Reference == reservation.Reference && saved.Handle == process.Handle {
			return ErrRunConflict
		}
	}
	reservation.Handle = process.Handle
	if process.Runtime == nil {
		b.Reservations[key] = reservation
		return nil
	}
	timing := process.Runtime
	if timing.Version != 1 || timing.ElapsedNS < 0 || timing.ElapsedNS < reservation.MeasuredNS {
		return ErrRunConflict
	}
	if timing.Settled {
		if receipt.State != "complete" && receipt.State != "dispatched" || process.ExitCode == nil {
			return ErrRunConflict
		}
		switch process.State {
		case "exited", "failed", "cancelled", "timed_out":
		default:
			return ErrRunConflict
		}
	}
	return settleProcessReservation(b, key, reservation, timing.ElapsedNS, timing.Settled && execution.ProcessDescendantsSettled(process))
}

func settleProcessReservation(b *ProcessBudget, key string, r ProcessReservation, elapsed int64, settled bool) error {
	if r.Settled {
		if !settled || r.MeasuredNS != elapsed {
			return ErrRunConflict
		}
		return nil
	}
	charge := max(r.ReservedNS, elapsed)
	if settled {
		charge = elapsed
	}
	delta := elapsed - r.MeasuredNS
	if delta < 0 || b.MeasuredNS < 0 || delta > math.MaxInt64-b.MeasuredNS || b.ChargedNS < r.ChargedNS || charge > math.MaxInt64-(b.ChargedNS-r.ChargedNS) || b.UnsettledStarts < 1 {
		return ErrRunConflict
	}
	b.MeasuredNS += delta
	b.ChargedNS = b.ChargedNS - r.ChargedNS + charge
	if settled {
		b.UnsettledStarts--
	}
	r.ChargedNS, r.MeasuredNS, r.Settled = charge, elapsed, settled
	b.Reservations[key] = r
	return nil
}

// RecordProcessObservation is called only by the authenticated host adapter.
// It records costs for an existing owned start; it creates neither grants nor
// reservations. Checkpoint and binding checks prevent cross-run settlement.
func (m *Manager) RecordProcessObservation(id, actor string, request execution.Request, response execution.Response) error {
	if request.Operation != "process_inspect" && request.Operation != "process_wait" && request.Operation != "process_stop" {
		return ErrRunConflict
	}
	if request.Token != "" || response.Version != execution.ProtocolVersion || response.ID != request.ID || response.Error != "" || response.Receipt == nil || len(response.Receipt.Result) > execution.MaxFrameBytes {
		return ErrRunConflict
	}
	_, err := m.updateTask(id, func(t *Task) error {
		cp, ref := t.Checkpoint, t.Config.Execution
		if t.Actor != actor || cp == nil || ref == nil || cp.ExecutingCall != request.ID || len(cp.Calls) == 0 || cp.Calls[0].ID != request.ID || execution.Operation(cp.Calls[0].Function.Name) != request.Operation || request.Binding.Actor != actor || request.Binding.Job != id || request.Binding.Host != ref.Host || request.Binding.Session != ref.Session || request.Binding.Grant != ref.Grant || request.Binding.Revision != ref.Revision {
			return ErrRunConflict
		}
		expected, ok := execution.Descriptor(cp.Calls[0].Function.Name)
		if !ok || cp.ExecutingCapability == nil || *cp.ExecutingCapability != expected {
			return ErrRunConflict
		}
		args, e := CanonicalArguments([]byte(cp.Calls[0].Function.Arguments))
		actual, e2 := CanonicalArguments(request.Arguments)
		if e != nil || e2 != nil || string(args) != string(actual) {
			return ErrRunConflict
		}
		return recordProcessReceipt(t, request, *response.Receipt)
	})
	return err
}
