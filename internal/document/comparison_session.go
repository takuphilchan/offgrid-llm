package document

import (
	"context"
	"fmt"
)

// PreparedComparison verifies immutable graphs once for a multi-unit job, rather
// than rehashing both complete documents at every page checkpoint. Callers must
// retain exclusive immutable ownership of the supplied extraction values.
type PreparedComparison struct {
	before, after *Extraction
	pairs         []ComparisonPair
}

func PrepareComparison(before, after *Extraction) (*PreparedComparison, error) {
	pairs, err := PairComparisonUnits(before, after)
	if err != nil {
		return nil, err
	}
	return &PreparedComparison{before: before, after: after, pairs: pairs}, nil
}

func (c *PreparedComparison) Step(ctx context.Context, ordinal int, expected ComparisonPair) (*UnitComparison, error) {
	if c == nil || ordinal < 0 || ordinal >= len(c.pairs) || c.pairs[ordinal] != expected {
		return nil, fmt.Errorf("comparison checkpoint identity mismatch")
	}
	return compareUnit(ctx, c.before, c.after, expected)
}

func CompareUnit(ctx context.Context, before, after *Extraction, pair ComparisonPair) (*UnitComparison, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prepared, err := PrepareComparison(before, after)
	if err != nil {
		return nil, err
	}
	for i, candidate := range prepared.pairs {
		if candidate == pair {
			return prepared.Step(ctx, i, pair)
		}
	}
	return nil, fmt.Errorf("comparison alignment does not match source revisions")
}
