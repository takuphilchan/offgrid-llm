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
		{"offgrid kb structure ID", "Inspect revision-pinned blocks and source relationships"},
		{"offgrid kb correct REVIEW.json", "Publish attributed corrections with revision checks"},
		{"offgrid kb import-attachment SESSION FILE_ID SHA256 COLLECTION_ID REVIEWED_REVISION", "Import a saved attachment"},
		{"offgrid kb ocr-languages", "Inspect local OCR language readiness"},
		{"offgrid kb import-ocr-language LANGUAGE PATH", "Import an OCR language pack"},
	})
	p.ModernSection("Manage", [][]string{
		{"offgrid kb collections", "List collections you can access"},
		{"offgrid kb collection ID", "Inspect the current audience and revision"},
		{"offgrid kb collection-audit ID [--after N]", "Read the owner's content-free collection audit"},
		{"offgrid kb create-collection NAME", "Create a private collection"},
		{"offgrid kb import-collection ID REVISION PATH [--ocr-language LANGUAGE]", "Retain a file and its OCR selection under the reviewed audience; use retry-import to process"},
		{"offgrid kb update-collection ID REVIEW.json", "Apply an owner-reviewed audience revision"},
		{"offgrid kb document-collection ID", "Inspect a document's collection binding"},
		{"offgrid kb move-document REVIEW.json", "Move a document with both audience revisions checked"},
		{"offgrid kb disable", "Disable knowledge indexing"},
		{"offgrid kb remove ID", "Remove an accessible document"},
		{"offgrid kb remove-import ID", "Remove a saved import"},
		{"offgrid kb clear --yes", "Explicitly remove all accessible documents"},
		{"offgrid kb retention ID", "Inspect retention and legal hold"},
	})
	p.ModernSection("Document analyses", [][]string{
		{"offgrid kb synthesise REQUEST.json", "Prepare summary or requirement drafts with an installed local model"},
		{"offgrid kb syntheses", "List private analysis history"},
		{"offgrid kb synthesis ID", "Inspect traversal progress and coverage"},
		{"offgrid kb synthesis-results ID [OFFSET]", "Review draft claims and exact source quotations"},
		{"offgrid kb retry-synthesis ID", "Explicitly resume after the last committed batch"},
		{"offgrid kb cancel-analysis ID", "Stop a comparison or synthesis job"},
		{"offgrid kb synthesis-export ID csv --output NEW_FILE", "Save a draft review table without overwriting"},
		{"offgrid kb synthesis-review ID BATCH", "Read saved review decisions for a source section"},
		{"offgrid kb save-synthesis-review REQUEST.json", "Save a pinned review with a revision check"},
		{"offgrid kb compose-summary REQUEST.json", "Compose a draft summary from pinned source sections"},
		{"offgrid kb compositions ANALYSIS_ID", "List saved summary reports"},
		{"offgrid kb composition-report ID", "Read the report with coverage and review status"},
		{"offgrid kb composition-evidence ID SECTION", "Inspect original evidence for a report section"},
		{"offgrid kb cancel-composition ID", "Stop summary composition"},
		{"offgrid kb retry-composition ID", "Resume after the last committed merge"},
		{"offgrid kb composition-export ID markdown --output NEW_FILE", "Export a draft with source references and gaps"},
	})
	p.Note("Retention changes: --revision N --delete-after UTC_UNIX_SECONDS --legal-hold true|false.")
	p.Note("Uses OFFGRID_SERVER_URL and OFFGRID_API_KEY. --json emits one result; Ctrl+C cancels the request.")
}
