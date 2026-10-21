package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
)

func auditCommand(_ context.Context, args []string, env Environment) error {
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	dataDir := cfg.DataDir
	auditDir := filepath.Join(dataDir, "audit")

	// Check for help flag or no args
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Audit Log%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sSecurity audit logging for compliance and monitoring%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sCommands%s\n", env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sShow recent audit events%s\n", "show", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sExport to JSON (compliance format)%s\n", "export-json <file>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sExport to CSV (spreadsheet format)%s\n", "export-csv <file>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sShow audit statistics%s\n", "stats", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sVerify log chain integrity%s\n", "verify", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sFilters%s\n", env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sFilter by event type (AUTH, QUERY, MODEL, etc.)%s\n", "--type <type>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sFilter by severity (INFO, WARNING, CRITICAL)%s\n", "--severity <level>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sFilter by username%s\n", "--user <name>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sStart date (YYYY-MM-DD)%s\n", "--from <date>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sEnd date (YYYY-MM-DD)%s\n", "--to <date>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-22s %sLimit number of results%s\n", "--limit <n>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid audit show --type AUTH --limit 50\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid audit export-csv report.csv --from 2025-01-01\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid audit stats --severity CRITICAL\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return nil
	}

	// Create audit logger
	auditCfg := audit.DefaultAuditConfig(dataDir)
	if env.LocalAdmin.NewAuditLogger == nil {
		return fmt.Errorf("local audit adapter unavailable")
	}
	logger, err := env.LocalAdmin.NewAuditLogger(auditCfg)
	if err != nil {
		presenter.Error(fmt.Sprintf("Failed to open audit log: %v", err))
		return LegacyReportedError{}
	}
	defer logger.Close()

	// Parse filter flags
	filter := audit.QueryFilter{Limit: 100}
	subCmd := args[0]
	args = args[1:]

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 < len(args) {
				filter.Types = append(filter.Types, audit.EventType(strings.ToUpper(args[i+1])))
				i++
			}
		case "--severity":
			if i+1 < len(args) {
				filter.Severities = append(filter.Severities, audit.EventSeverity(strings.ToUpper(args[i+1])))
				i++
			}
		case "--user":
			if i+1 < len(args) {
				filter.User = args[i+1]
				i++
			}
		case "--from":
			if i+1 < len(args) {
				if t, err := time.Parse("2006-01-02", args[i+1]); err == nil {
					filter.StartTime = t
				}
				i++
			}
		case "--to":
			if i+1 < len(args) {
				if t, err := time.Parse("2006-01-02", args[i+1]); err == nil {
					filter.EndTime = t.Add(24 * time.Hour) // End of day
				}
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &filter.Limit)
				i++
			}
		}
	}

	switch subCmd {
	case "show", "list":
		events, err := logger.Query(filter)
		if err != nil {
			presenter.Error(fmt.Sprintf("Query failed: %v", err))
			return LegacyReportedError{}
		}

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Audit Events%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %s%d events%s\n", env.Style.Muted, len(events), env.Style.Reset)
		fmt.Fprintln(env.Stdout)

		if len(events) == 0 {
			fmt.Fprintf(env.Stdout, "    %sNo events found%s\n", env.Style.Muted, env.Style.Reset)
			fmt.Fprintln(env.Stdout)
			return nil
		}

		// Print table header
		fmt.Fprintf(env.Stdout, "    %s%-20s  %-8s  %-8s  %-20s  %-15s%s\n",
			env.Style.Muted, "TIMESTAMP", "TYPE", "SEVERITY", "ACTION", "USER", env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s%s%s\n", env.Style.Muted, strings.Repeat("─", 75), env.Style.Reset)

		for _, event := range events {
			// Parse and format timestamp
			ts := event.Timestamp
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				ts = t.Format("01-02 15:04:05")
			}

			// Color code severity
			sevColor := env.Style.Muted
			switch event.Severity {
			case audit.SeverityWarning:
				sevColor = env.Style.Accent
			case audit.SeverityCritical:
				sevColor = env.Style.Error
			}

			// Truncate action if needed
			action := event.Action
			if len(action) > 20 {
				action = action[:17] + "..."
			}

			user := event.User
			if user == "" {
				user = "-"
			}

			fmt.Fprintf(env.Stdout, "    %-20s  %-8s  %s%-8s%s  %-20s  %-15s\n",
				ts, event.Type, sevColor, event.Severity, env.Style.Reset, action, user)
		}
		fmt.Fprintln(env.Stdout)

	case "export-json":
		if len(args) == 0 || strings.HasPrefix(args[0], "--") {
			presenter.Error("Usage: offgrid audit export-json <output-file>")
			return LegacyReportedError{}
		}
		outputPath := args[0]
		if !strings.HasSuffix(outputPath, ".json") {
			outputPath += ".json"
		}

		if err := logger.ExportForCompliance(outputPath, filter); err != nil {
			presenter.Error(fmt.Sprintf("Export failed: %v", err))
			return LegacyReportedError{}
		}

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s%s%s Exported to %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, outputPath)
		fmt.Fprintln(env.Stdout)

	case "export-csv":
		if len(args) == 0 || strings.HasPrefix(args[0], "--") {
			presenter.Error("Usage: offgrid audit export-csv <output-file>")
			return LegacyReportedError{}
		}
		outputPath := args[0]
		if !strings.HasSuffix(outputPath, ".csv") {
			outputPath += ".csv"
		}

		if err := logger.ExportToCSV(outputPath, filter); err != nil {
			presenter.Error(fmt.Sprintf("Export failed: %v", err))
			return LegacyReportedError{}
		}

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s%s%s Exported to %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, outputPath)
		fmt.Fprintln(env.Stdout)

	case "stats":
		stats, err := logger.GetStats(filter)
		if err != nil {
			presenter.Error(fmt.Sprintf("Failed to get stats: %v", err))
			return LegacyReportedError{}
		}

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Audit Statistics%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)

		fmt.Fprintf(env.Stdout, "    %sTotal Events%s     %d\n", env.Style.Bold, env.Style.Reset, stats.TotalEvents)
		fmt.Fprintf(env.Stdout, "    %sSuccessful%s       %s%d%s\n", env.Style.Bold, env.Style.Reset, env.Style.Success, stats.SuccessCount, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %sFailed%s           %s%d%s\n", env.Style.Bold, env.Style.Reset, env.Style.Error, stats.FailureCount, env.Style.Reset)
		fmt.Fprintln(env.Stdout)

		if len(stats.ByType) > 0 {
			fmt.Fprintf(env.Stdout, "    %sBy Type%s\n", env.Style.Bold, env.Style.Reset)
			for t, count := range stats.ByType {
				fmt.Fprintf(env.Stdout, "      %-12s  %d\n", t, count)
			}
			fmt.Fprintln(env.Stdout)
		}

		if len(stats.BySeverity) > 0 {
			fmt.Fprintf(env.Stdout, "    %sBy Severity%s\n", env.Style.Bold, env.Style.Reset)
			for s, count := range stats.BySeverity {
				fmt.Fprintf(env.Stdout, "      %-12s  %d\n", s, count)
			}
			fmt.Fprintln(env.Stdout)
		}

		if len(stats.ByUser) > 0 {
			fmt.Fprintf(env.Stdout, "    %sBy User%s\n", env.Style.Bold, env.Style.Reset)
			for u, count := range stats.ByUser {
				fmt.Fprintf(env.Stdout, "      %-12s  %d\n", u, count)
			}
			fmt.Fprintln(env.Stdout)
		}

	case "verify":
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Verifying Audit Chain Integrity%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)

		if env.LocalAdmin.GlobAuditFiles == nil {
			return fmt.Errorf("local audit file adapter unavailable")
		}
		files, _ := env.LocalAdmin.GlobAuditFiles(filepath.Join(auditDir, "audit_*.jsonl"))
		if len(files) == 0 {
			fmt.Fprintf(env.Stdout, "    %sNo audit logs found%s\n", env.Style.Muted, env.Style.Reset)
			fmt.Fprintln(env.Stdout)
			return nil
		}

		totalVerified := 0
		var allErrors []string

		for _, f := range files {
			verified, errors, _ := logger.VerifyChain(f)
			totalVerified += verified
			allErrors = append(allErrors, errors...)

			basename := filepath.Base(f)
			if len(errors) == 0 {
				fmt.Fprintf(env.Stdout, "    %s%s%s  %s (%d events)\n", env.Style.Success, env.Style.Check, env.Style.Reset, basename, verified)
			} else {
				fmt.Fprintf(env.Stdout, "    %s%s%s  %s (%d errors)\n", env.Style.Error, env.Style.Cross, env.Style.Reset, basename, len(errors))
			}
		}

		fmt.Fprintln(env.Stdout)
		if len(allErrors) == 0 {
			fmt.Fprintf(env.Stdout, "  %s%s All %d events verified - chain integrity intact%s\n",
				env.Style.Success, env.Style.Check, totalVerified, env.Style.Reset)
		} else {
			fmt.Fprintf(env.Stdout, "  %s%s %d integrity errors detected%s\n",
				env.Style.Error, env.Style.Cross, len(allErrors), env.Style.Reset)
			for _, e := range allErrors {
				fmt.Fprintf(env.Stdout, "    %s• %s%s\n", env.Style.Error, e, env.Style.Reset)
			}
		}
		fmt.Fprintln(env.Stdout)

	default:
		presenter.Error(fmt.Sprintf("Unknown audit command: %s", subCmd))
		return LegacyReportedError{}
	}
	return nil
}

func RunAuditCommand(ctx context.Context, args []string, env Environment) error {
	return auditCommand(ctx, args, env)
}
