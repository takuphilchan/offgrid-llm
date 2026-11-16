package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

const KnowledgeSynthesisUsage = "synthesise REQUEST.json|syntheses [OFFSET]|synthesis ID|synthesis-results ID [OFFSET]|retry-synthesis ID|synthesis-review ID BATCH|save-synthesis-review REQUEST.json|synthesis-export ID json|csv|markdown --output NEW_FILE|" + KnowledgeCompositionUsage

func runKnowledgeSynthesis(ctx context.Context, c *serviceclient.Client, args []string) (any, error) {
	if args[0] == "synthesise" && len(args) == 2 {
		f, err := os.Open(args[1])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if !stat.Mode().IsRegular() || stat.Size() > 16384 {
			return nil, Usage("Choose a regular analysis request file no larger than 16 KiB")
		}
		var body serviceclient.DocumentSynthesisRequest
		decoder := json.NewDecoder(io.LimitReader(f, 16385))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
			return nil, Usage("Provide request_key, workflow, sources, model and language")
		}
		return c.CreateDocumentSynthesis(ctx, body)
	}
	if args[0] == "synthesis" && len(args) == 2 {
		return c.DocumentSynthesis(ctx, args[1])
	}
	if args[0] == "retry-synthesis" && len(args) == 2 {
		err := c.ControlDocumentSynthesis(ctx, args[1], "retry")
		return map[string]bool{"accepted": err == nil}, err
	}
	if args[0] == "syntheses" && len(args) <= 2 {
		offset := 0
		if len(args) == 2 {
			v, err := strconv.Atoi(args[1])
			if err != nil {
				return nil, Usage("OFFSET must be an integer")
			}
			offset = v
		}
		return c.DocumentSynthesisHistory(ctx, offset)
	}
	if args[0] == "synthesis-results" && (len(args) == 2 || len(args) == 3) {
		offset := 0
		if len(args) == 3 {
			v, err := strconv.Atoi(args[2])
			if err != nil {
				return nil, Usage("OFFSET must be an integer")
			}
			offset = v
		}
		return c.DocumentSynthesisResults(ctx, args[1], offset)
	}
	return nil, Usage("Usage: offgrid kb %s", KnowledgeSynthesisUsage)
}
