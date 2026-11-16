// Package document defines source evidence shared by Chat and Knowledge.
package document

type Warning struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	SourceIndex int    `json:"source_index,omitempty"`
}

// Coverage measures extraction, not factual correctness or successful indexing.
type Coverage struct {
	Format        string    `json:"format,omitempty"`
	Status        string    `json:"status"`
	TotalUnits    int       `json:"total_units"`
	ReadableUnits int       `json:"readable_units"`
	ReviewUnits   []int     `json:"review_units,omitempty"`
	Warnings      []Warning `json:"warnings,omitempty"`
}
