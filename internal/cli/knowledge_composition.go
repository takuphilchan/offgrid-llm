package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

const KnowledgeCompositionUsage = "compose-summary REQUEST.json|compositions ANALYSIS_ID [OFFSET]|composition ID|composition-report ID|composition-evidence ID SECTION [OFFSET]|cancel-composition ID|retry-composition ID|composition-export ID json|csv|markdown --output NEW_FILE"

func runKnowledgeComposition(ctx context.Context, c *serviceclient.Client, args []string) (any, error) {
	if args[0] == "compose-summary" && len(args) == 2 {
		f, err := os.Open(args[1])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 4096 {
			return nil, Usage("Choose a regular composition request file no larger than 4 KiB")
		}
		var body serviceclient.DocumentCompositionRequest
		d := json.NewDecoder(io.LimitReader(f, 4097))
		d.DisallowUnknownFields()
		if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF {
			return nil, Usage("Provide request_key, analysis_id, model and language")
		}
		return c.CreateDocumentComposition(ctx, body)
	}
	if len(args) == 2 {
		switch args[0] {
		case "composition":
			return c.DocumentComposition(ctx, args[1])
		case "composition-report":
			return c.DocumentCompositionReport(ctx, args[1])
		case "cancel-composition", "retry-composition":
			err := c.ControlDocumentComposition(ctx, args[1], strings.TrimSuffix(args[0], "-composition"))
			return map[string]bool{"accepted": err == nil}, err
		}
	}
	if args[0] == "compositions" && (len(args) == 2 || len(args) == 3) {
		offset := 0
		if len(args) == 3 {
			v, err := strconv.Atoi(args[2])
			if err != nil {
				return nil, Usage("OFFSET must be an integer")
			}
			offset = v
		}
		return c.DocumentCompositionHistory(ctx, args[1], offset)
	}
	if args[0] == "composition-evidence" && (len(args) == 3 || len(args) == 4) {
		section, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, Usage("SECTION must be an integer")
		}
		offset := 0
		if len(args) == 4 {
			offset, err = strconv.Atoi(args[3])
			if err != nil {
				return nil, Usage("OFFSET must be an integer")
			}
		}
		return c.DocumentCompositionEvidence(ctx, args[1], section, offset)
	}
	return nil, Usage("Usage: offgrid kb %s", KnowledgeCompositionUsage)
}
