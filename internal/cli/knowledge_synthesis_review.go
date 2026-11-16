package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func runKnowledgeSynthesisReview(ctx context.Context, c *serviceclient.Client, args []string) (any, error) {
	if args[0] == "synthesis-review" && (len(args) == 3 || len(args) == 4) {
		batch, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, Usage("BATCH must be an integer")
		}
		version := 0
		if len(args) == 4 {
			version, err = strconv.Atoi(args[3])
			if err != nil {
				return nil, Usage("REVISION must be an integer")
			}
		}
		return c.DocumentSynthesisReview(ctx, args[1], batch, version)
	}
	if args[0] == "save-synthesis-review" && len(args) == 2 {
		f, err := os.Open(args[1])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if !stat.Mode().IsRegular() || stat.Size() > 128<<10 {
			return nil, Usage("Choose a regular review request file no larger than 128 KiB")
		}
		var body rag.SynthesisReviewRequest
		d := json.NewDecoder(io.LimitReader(f, (128<<10)+1))
		d.DisallowUnknownFields()
		if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF {
			return nil, Usage("Provide the pinned section, expected_revision, section_checked and decisions")
		}
		return c.SaveDocumentSynthesisReview(ctx, body)
	}
	return nil, Usage("Usage: offgrid kb synthesis-review ID BATCH [REVISION] | save-synthesis-review REQUEST.json")
}
