package api

// Public inspection projections deliberately exclude execution grants, private
// working-copy locations, process environments and displaced-file store paths.
type WorkChange struct {
	ID                 string `json:"id"`
	WorkID             string `json:"work_id"`
	RunID              string `json:"run_id"`
	WorkerRunID        string `json:"worker_run_id"`
	Kind               string `json:"kind"`
	State              string `json:"state"`
	InstructionVersion uint64 `json:"instruction_version"`
	ControlEpoch       uint64 `json:"control_epoch"`
	Current            bool   `json:"current"`
	OperationCount     int    `json:"operation_count"`
	ValidationState    string `json:"validation_state"`
	ValidationCount    int    `json:"validation_count"`
	IntegrationID      string `json:"integration_id,omitempty"`
	IntegratedCount    int    `json:"integrated_count"`
	Available          bool   `json:"available"`
	Limitation         string `json:"limitation,omitempty"`
}

type WorkChangeList struct {
	Items      []WorkChange `json:"items"`
	NextCursor string       `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

type WorkFileVersion struct {
	Kind   string `json:"kind"`
	Bytes  int64  `json:"bytes"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256,omitempty"`
}

type WorkFileChange struct {
	Path   string           `json:"path"`
	Kind   string           `json:"kind"`
	Base   *WorkFileVersion `json:"base,omitempty"`
	Result *WorkFileVersion `json:"result,omitempty"`
}

type WorkValidationCheck struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	ExitCode      *int   `json:"exit_code,omitempty"`
	RequestDigest string `json:"request_digest"`
	ReceiptDigest string `json:"receipt_digest"`
}

type WorkFileReceipt struct {
	Ordinal          int    `json:"ordinal"`
	Path             string `json:"path"`
	Operation        string `json:"operation"`
	State            string `json:"state"`
	Dispatched       bool   `json:"dispatched"`
	OriginalRetained bool   `json:"original_retained"`
	DisplacedDigest  string `json:"displaced_digest,omitempty"`
	ResultDigest     string `json:"result_digest,omitempty"`
	Error            string `json:"error,omitempty"`
}

type WorkChangeContent struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"`
}

type WorkRecoveryFile struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Role       string `json:"role"`
	SHA256     string `json:"sha256,omitempty"`
	Bytes      int64  `json:"bytes"`
	Limitation string `json:"limitation,omitempty"`
}

type WorkChangeDetail struct {
	Change       WorkChange            `json:"change"`
	View         string                `json:"view"`
	Files        []WorkFileChange      `json:"files,omitempty"`
	Checks       []WorkValidationCheck `json:"checks,omitempty"`
	Receipts     []WorkFileReceipt     `json:"receipts,omitempty"`
	Recovery     []WorkRecoveryFile    `json:"recovery,omitempty"`
	Content      *WorkChangeContent    `json:"content,omitempty"`
	NextOffset   *int64                `json:"next_offset,omitempty"`
	ConflictPath string                `json:"conflict_path,omitempty"`
	Error        string                `json:"error,omitempty"`
}
