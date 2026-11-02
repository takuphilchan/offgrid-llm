package rag

import "strings"

// Keep selected IDs as bound values, and use the same predicate for keyword
// and semantic candidates before ranking. Selection itself grants no access.
func sqliteDocumentSelection(ids []string) (string, []any) {
	if len(ids) == 0 {
		return "", nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return " AND c.document_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")", args
}
