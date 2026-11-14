package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ReplacementRecovery is evidence, not a new executable request. Private file
// paths and grants stay in the supervisor. Unknown dispatch is never promoted
// to success merely because a target currently has the desired bytes.
type ReplacementRecovery struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Expected  string `json:"expected_sha256"`
	Proposed  string `json:"proposed_sha256"`
	Displaced string `json:"displaced_sha256,omitempty"`
}

func (j *Journal) replacementRecovery(ctx context.Context, operation string) (*ReplacementRecovery, error) {
	var evidence ReplacementRecovery
	var encoded []byte
	err := j.db.QueryRowContext(ctx, `SELECT directory,state,expected,proposed,receipt FROM execution_replacements WHERE operation=?`, operation).Scan(&evidence.ID, &evidence.State, &evidence.Expected, &evidence.Proposed, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(encoded) > 0 {
		var actual preservedReplacement
		if err = json.Unmarshal(encoded, &actual); err != nil {
			return nil, err
		}
		evidence.Displaced = actual.DisplacedDigest
	}
	return &evidence, nil
}
