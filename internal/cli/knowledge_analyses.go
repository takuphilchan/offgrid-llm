package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

const KnowledgeAnalysisUsage = "compare REQUEST.json|analyses [OFFSET [LIMIT]]|analysis ID|analysis-results ID STEP [OFFSET [LIMIT]]|analysis-source ID STEP CHANGE before|after|analysis-export ID json|csv|markdown --output NEW_FILE|cancel-analysis ID|retry-analysis ID|" + KnowledgeSynthesisUsage

func runKnowledgeAnalyses(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	if args[0] == "analyses" && len(args) <= 3 {
		offset, limit := 0, 20
		for i, target := range []*int{&offset, &limit} {
			if len(args) > i+1 {
				v, e := strconv.Atoi(args[i+1])
				if e != nil {
					return nil, Usage("OFFSET and LIMIT must be integers")
				}
				*target = v
			}
		}
		return client.DocumentAnalysisHistory(ctx, offset, limit)
	}
	if args[0] == "compare" {
		if len(args) != 2 {
			return nil, Usage("Usage: offgrid kb compare REQUEST.json")
		}
		file, err := os.Open(args[1])
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 16384 {
			return nil, Usage("Choose a regular comparison request file no larger than 16 KiB")
		}
		data, err := io.ReadAll(io.LimitReader(file, 16385))
		if err != nil {
			return nil, err
		}
		if len(data) > 16384 {
			return nil, Usage("Comparison request exceeds 16 KiB")
		}
		var request struct {
			RequestKey string               `json:"request_key"`
			Sources    []document.SourceRef `json:"sources"`
		}
		d := json.NewDecoder(bytes.NewReader(data))
		d.DisallowUnknownFields()
		if d.Decode(&request) != nil || d.Decode(new(any)) != io.EOF {
			return nil, Usage("Provide request_key and two ordered sources with document_id, revision and original_sha256")
		}
		return client.CreateDocumentComparison(ctx, request.RequestKey, request.Sources)
	}
	if len(args) == 2 {
		switch args[0] {
		case "analysis":
			return client.DocumentAnalysis(ctx, args[1])
		case "cancel-analysis", "retry-analysis":
			action := "cancel"
			if args[0] == "retry-analysis" {
				action = "retry"
			}
			err := client.ControlDocumentAnalysis(ctx, args[1], action)
			return map[string]bool{"accepted": err == nil}, err
		}
	}
	if args[0] == "analysis-results" && len(args) >= 3 && len(args) <= 5 {
		step, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, Usage("STEP must be an integer")
		}
		offset, limit := 0, 100
		if len(args) > 3 {
			offset, err = strconv.Atoi(args[3])
			if err != nil {
				return nil, Usage("OFFSET must be an integer")
			}
		}
		if len(args) > 4 {
			limit, err = strconv.Atoi(args[4])
			if err != nil {
				return nil, Usage("LIMIT must be an integer")
			}
		}
		return client.DocumentAnalysisResults(ctx, args[1], step, offset, limit)
	}
	if args[0] == "analysis-source" && len(args) == 5 {
		step, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, Usage("STEP must be an integer")
		}
		change, err := strconv.Atoi(args[3])
		if err != nil {
			return nil, Usage("CHANGE must be an integer")
		}
		return client.DocumentAnalysisSource(ctx, args[1], step, change, args[4])
	}
	return nil, Usage("Usage: offgrid kb compare REQUEST.json | analysis ID | analysis-results ID STEP [OFFSET [LIMIT]] | analysis-source ID STEP CHANGE before|after | cancel-analysis ID | retry-analysis ID")
}
