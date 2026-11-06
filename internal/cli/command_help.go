package cli

func (p Presenter) modelPackageHelp() {
	p.Heading("OffGrid / Models")
	p.ModernSection("Find and review", [][]string{
		{"offgrid model list", "Installed packages and runtime readiness"},
		{"offgrid model catalog", "Available model packages"},
		{"offgrid model search QUERY --category asr|tts", "Find speech models"},
		{"offgrid model discover OWNER/REPO", "Inspect repository variants and recipes"},
		{"offgrid model preview ID", "Review files, size and license notices"},
	})
	p.ModernSection("Install and recover", [][]string{
		{"offgrid model install ID --yes", "Approve the previewed package download"},
		{"offgrid model status [OPERATION_ID]", "Inspect saved downloads"},
		{"offgrid model cancel OPERATION_ID", "Stop a download"},
		{"offgrid model resume OPERATION_ID", "Continue a saved download"},
		{"offgrid model discard OPERATION_ID --yes", "Remove retained download data"},
		{"offgrid model repair OPERATION_ID --yes", "Repair from a completed source operation"},
		{"offgrid model verify ID --revision REVISION", "Check an installed revision"},
		{"offgrid model remove ID --revision REVISION --yes", "Remove that installed revision"},
	})
	p.Heading("Options")
	p.Note("List/catalog: --category language|embeddings|asr|tts.")
	p.Note("Repository preview/install: --revision COMMIT --variant FILE --architecture RECIPE, using values from discover.")
	p.Note("Install: --detach submits without waiting; --request-id ID preserves request identity.")
	p.Note("Uses OFFGRID_SERVER_URL and OFFGRID_API_KEY. A package install does not install or qualify its runtime. Detailed results retain their structured format.")
}

func (p Presenter) audioHelp() {
	p.Heading("OffGrid / Speech")
	p.ModernSection("Use speech", [][]string{
		{"offgrid audio status", "Inspect speech readiness"},
		{"offgrid audio models", "List speech models"},
		{"offgrid audio transcribe FILE", "Transcribe a recording"},
		{"offgrid audio speak TEXT --output FILE", "Generate a speech file"},
	})
	p.ModernSection("Set up", [][]string{
		{"offgrid audio setup whisper --model NAME", "Download a supported recognition model"},
		{"offgrid audio setup piper --voice NAME", "Download a supported voice"},
	})
	p.Heading("Options")
	p.rows([]presentationRow{{"--model NAME", "Recognition model"}, {"--voice NAME", "Piper voice"}, {"--language CODE", "Recognition language"}, {"--output FILE", "Speech output destination"}})
	p.Note("Setup downloads model assets. A matching local runtime must also be available.")
}

func (p Presenter) usersHelp() {
	p.Heading("OffGrid / Users")
	p.Note("Local administration for a service configured with multi_user_mode: true.")
	p.ModernSection("Inspect", [][]string{{"offgrid users list", "List users"}, {"offgrid users info ID", "Inspect one user"}, {"offgrid users quota ID", "Inspect quota usage"}})
	p.ModernSection("Manage access", [][]string{{"offgrid users create NAME ROLE", "Create a user"}, {"offgrid users delete ID", "Delete a user"}, {"offgrid users reset-key ID", "Regenerate an API key"}})
	p.Note("Roles: admin, user, viewer, guest. Key output is sensitive; do not share it.")
}

func (p Presenter) auditHelp() {
	p.Heading("OffGrid / Audit")
	p.ModernSection("Inspect and export", [][]string{
		{"offgrid audit show", "Read recent audit events"},
		{"offgrid audit stats", "Summarize recorded events"},
		{"offgrid audit verify", "Verify log-chain integrity"},
		{"offgrid audit export-json FILE", "Export structured events"},
		{"offgrid audit export-csv FILE", "Export a spreadsheet-compatible file"},
	})
	p.Heading("Filters")
	p.rows([]presentationRow{{"--type TYPE", "AUTH, QUERY, MODEL, and other event types"}, {"--severity LEVEL", "INFO, WARNING, CRITICAL"}, {"--user NAME", "Filter by username"}, {"--from YYYY-MM-DD", "Start date"}, {"--to YYYY-MM-DD", "End date"}, {"--limit N", "Maximum results"}})
	p.Note("These commands inspect the configured local audit store. Exports may contain private activity.")
}
