package cli

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"os"
)

func exportKnowledgeAnalysis(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	if len(args) != 5 || args[3] != "--output" || args[4] == "" {
		return nil, Usage("Usage: offgrid kb analysis-export|synthesis-export ID json|csv|markdown --output NEW_FILE")
	}
	export := client.ExportDocumentAnalysis
	if args[0] == "synthesis-export" {
		export = client.ExportDocumentSynthesis
	}
	if args[0] == "composition-export" {
		export = client.ExportDocumentComposition
	}
	artifact, err := export(ctx, args[1], args[2])
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	file, err := os.OpenFile(args[4], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		file.Close()
		if !ok {
			os.Remove(args[4])
		}
	}()
	if _, err = file.WriteString(artifact.Content); err != nil {
		return nil, err
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	ok = true
	return map[string]any{"success": true, "analysis_id": artifact.AnalysisID, "format": artifact.Format, "output": args[4], "bytes": len(artifact.Content), "sha256": artifact.SHA256}, nil
}
