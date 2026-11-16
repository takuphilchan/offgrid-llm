package document

import (
	"crypto/sha256"
	"encoding/hex"
)

// PrepareSynthesis has verified the immutable graphs. Index their source units
// and blocks once: re-running graph.Verify for each quote makes long-document
// composition quadratic before any model work starts.
func compositionQuoteVerifier(graphs []*Extraction) func(SynthesisCitedQuote) bool {
	units := make([]map[string]Unit, len(graphs))
	blocks := make([]map[string]Block, len(graphs))
	for i, g := range graphs {
		units[i] = map[string]Unit{}
		blocks[i] = map[string]Block{}
		for _, u := range g.Units {
			units[i][u.ID] = u
		}
		for _, b := range g.Blocks {
			blocks[i][b.ID] = b
		}
	}
	return func(q SynthesisCitedQuote) bool {
		c := q.Citation
		if c.Source < 0 || c.Source >= len(graphs) || len(q.Text) == 0 || len(q.Text) > 8192 {
			return false
		}
		g := graphs[c.Source]
		if c.Revision != g.ID || !validSpan(g.Text, c.Span) || q.Text != g.Text[c.Span.Start:c.Span.End] {
			return false
		}
		u, ok := units[c.Source][c.UnitID]
		if !ok || c.Span.Start < u.Span.Start || c.Span.End > u.Span.End {
			return false
		}
		if c.BlockID != "" {
			b, ok := blocks[c.Source][c.BlockID]
			if !ok || b.UnitID != c.UnitID || c.Span.Start < b.Span.Start || c.Span.End > b.Span.End {
				return false
			}
		}
		hash := sha256.Sum256([]byte(q.Text))
		return c.TextSHA256 == hex.EncodeToString(hash[:])
	}
}
