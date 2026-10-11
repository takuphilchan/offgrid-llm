package computer

import (
	"context"
	"reflect"
	"time"
)

// NativeObservationRefresher is optional and used only by protocol-v3 hosts.
// It must actually re-observe expired structured state, not extend a timestamp
// or match a control by its label. Pointer/vision actions cannot use this path.
type NativeObservationRefresher interface {
	RefreshObservation(context.Context, ControlBinding, ControlObservation) (ControlObservation, error)
}

// matchingReobservation maps newly observed IDs to the old model-visible IDs
// only when complete views and the driver-computed runtime/state digest match.
// The fresh observation's identity and capture time remain new.
func matchingReobservation(old, fresh NativeView, now time.Time) (map[string]string, error) {
	if old.Limited || fresh.Limited || len(old.Elements) == 0 || len(old.Elements) != len(fresh.Elements) ||
		old.Observation.Validate(old.Observation.Target, old.Observation.CapturedAt) != nil ||
		fresh.Observation.Validate(old.Observation.Target, now) != nil ||
		!fresh.Observation.CapturedAt.After(old.Observation.CapturedAt) || fresh.Observation.Sequence <= old.Observation.Sequence ||
		fresh.Observation.ID == old.Observation.ID || fresh.Observation.StateDigest != old.Observation.StateDigest {
		return nil, ErrStaleObservation
	}
	ids := make(map[string]string, len(old.Elements))
	oldIDs := make(map[string]bool, len(old.Elements))
	for i, before := range old.Elements {
		after := fresh.Elements[i]
		if !opaqueID(before.ID) || !opaqueID(after.ID) || oldIDs[before.ID] || ids[after.ID] != "" {
			return nil, ErrStaleObservation
		}
		oldIDs[before.ID] = true
		ids[after.ID] = before.ID
		before.ID, after.ID = "", ""
		if !reflect.DeepEqual(before, after) {
			return nil, ErrStaleObservation
		}
	}
	return ids, nil
}
