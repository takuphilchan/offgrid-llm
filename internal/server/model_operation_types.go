package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

type ModelArtifactProgress struct {
	Path       string `json:"path"`
	BytesDone  int64  `json:"bytes_done"`
	BytesTotal int64  `json:"bytes_total"`
	Verified   bool   `json:"verified"`
}

// ModelOperation is persisted in downloads.json with legacy progress. Actor and
// request binding survive cancellation, restart and a lost acceptance reply.
type ModelOperation struct {
	ID               string                    `json:"id"`
	ActorID          string                    `json:"actor_id"`
	RequestID        string                    `json:"request_id"`
	RequestDigest    string                    `json:"request_digest"`
	Action           string                    `json:"action"`
	Target           models.InstallationTarget `json:"target"`
	Provenance       models.ModelProvenance    `json:"provenance"`
	State            string                    `json:"state"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
	Artifacts        []ModelArtifactProgress   `json:"artifacts"`
	BytesDone        int64                     `json:"bytes_done"`
	BytesTotal       int64                     `json:"bytes_total"`
	RetainedBytes    int64                     `json:"retained_bytes"`
	ErrorCode        string                    `json:"error_code,omitempty"`
	Message          string                    `json:"message,omitempty"`
	Discarded        bool                      `json:"discarded"`
	ActivationIntent bool                      `json:"activation_intent"`
}

var modelOperationID = regexp.MustCompile(`^model-[a-f0-9]{32}$`)
var modelRequestDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (o *ModelOperation) active() bool {
	return slices.Contains([]string{"queued", "downloading", "verifying", "activating", "cancelling"}, o.State)
}
func (o *ModelOperation) validate() error {
	if o == nil || !modelOperationID.MatchString(o.ID) || o.ActorID == "" || len(o.ActorID) > 256 || o.RequestID == "" || len(o.RequestID) > 128 || !modelRequestDigest.MatchString(o.RequestDigest) || !slices.Contains([]string{"install", "repair"}, o.Action) || !slices.Contains([]string{"queued", "downloading", "verifying", "activating", "complete", "cancelling", "cancelled", "interrupted", "failed"}, o.State) || o.CreatedAt.IsZero() || o.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid model operation identity or state")
	}
	if err := o.Target.Validate(); err != nil {
		return err
	}
	if o.Target.Kind != "package" {
		return fmt.Errorf("legacy transfers use their compatible single-file projection")
	}
	if err := models.ValidateAcquisitionManifest(*o.Target.Package); err != nil {
		return err
	}
	if o.Provenance.Kind != "repository_metadata" && o.Provenance.Kind != "curated_manifest" {
		return fmt.Errorf("invalid model source provenance")
	}
	if len(o.Artifacts) != len(o.Target.Package.Artifacts) {
		return fmt.Errorf("invalid artifact checkpoint count")
	}
	var total, done int64
	for i, a := range o.Target.Package.Artifacts {
		p := o.Artifacts[i]
		if p.Path != a.Path || p.BytesTotal != a.Size || p.BytesDone < 0 || p.BytesDone > a.Size || (p.Verified && p.BytesDone != a.Size) {
			return fmt.Errorf("invalid artifact checkpoint")
		}
		total += a.Size
		done += p.BytesDone
	}
	if o.BytesTotal != total || o.BytesDone != done || o.RetainedBytes < 0 || o.RetainedBytes > total*2 {
		return fmt.Errorf("invalid aggregate checkpoint")
	}
	if o.State == "complete" && (done != total || !o.ActivationIntent) {
		return fmt.Errorf("invalid completion record")
	}
	return nil
}

func modelPayloadDigest(value any) string {
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func cloneModelOperation(o *ModelOperation) *ModelOperation {
	if o == nil {
		return nil
	}
	b, _ := json.Marshal(o)
	var result ModelOperation
	_ = json.Unmarshal(b, &result)
	return &result
}
