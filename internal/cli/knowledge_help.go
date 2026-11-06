package cli

func (p Presenter) knowledgeHelp() {
	p.Heading("OffGrid / Knowledge")
	p.Note("Import documents, inspect preparation, and search their readable text.")
	p.ModernSection("Start here", [][]string{
		{"offgrid kb status", "Check indexing readiness"},
		{"offgrid kb enable MODEL", "Choose an installed embedding model"},
		{"offgrid kb add PATH", "Prepare a file or folder"},
		{"offgrid kb list", "List accessible documents"},
		{"offgrid kb search QUERY", "Search indexed passages"},
	})
	p.ModernSection("Preparation and sources", [][]string{
		{"offgrid kb imports", "List saved imports"},
		{"offgrid kb import-status ID", "Inspect preparation status"},
		{"offgrid kb review-import ID", "Review extracted content and limits"},
		{"offgrid kb retry-import ID", "Retry a saved import"},
		{"offgrid kb cancel-import ID", "Cancel preparation"},
		{"offgrid kb original ID --output NEW_FILE", "Save the original without overwriting"},
		{"offgrid kb import-attachment SESSION FILE_ID SHA256", "Import a saved attachment"},
		{"offgrid kb ocr-languages", "Inspect local OCR language readiness"},
		{"offgrid kb import-ocr-language LANGUAGE PATH", "Import an OCR language pack"},
	})
	p.ModernSection("Manage", [][]string{
		{"offgrid kb disable", "Disable knowledge indexing"},
		{"offgrid kb remove ID", "Remove an accessible document"},
		{"offgrid kb remove-import ID", "Remove a saved import"},
		{"offgrid kb clear --yes", "Explicitly remove all accessible documents"},
		{"offgrid kb retention ID", "Inspect retention and legal hold"},
	})
	p.Note("Retention changes: --revision N --delete-after UTC_UNIX_SECONDS --legal-hold true|false.")
	p.Note("Uses OFFGRID_SERVER_URL and OFFGRID_API_KEY. --json emits one result; Ctrl+C cancels the request.")
}
