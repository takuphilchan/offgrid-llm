package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func readCollectionReview(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 96<<10 {
		return Usage("Choose a regular collection review JSON file no larger than 96 KiB")
	}
	d := json.NewDecoder(io.LimitReader(f, (96<<10)+1))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		return Usage("Invalid collection review JSON; include explicit audience lists and current revisions")
	}
	return nil
}

func runKnowledgeCollections(ctx context.Context, c *serviceclient.Client, args []string) (any, error) {
	switch args[0] {
	case "collection-audit":
		if len(args) == 2 {
			return c.CollectionAudit(ctx, args[1], 0)
		}
		if len(args) == 4 && args[2] == "--after" {
			after, err := strconv.ParseInt(args[3], 10, 64)
			if err != nil || after < 0 {
				return nil, Usage("Use a nonnegative audit sequence")
			}
			return c.CollectionAudit(ctx, args[1], after)
		}
	case "import-collection":
		if len(args) == 4 || len(args) == 6 && args[4] == "--ocr-language" && args[5] != "" {
			revision, err := strconv.Atoi(args[2])
			if err != nil || revision < 1 || args[1] == "" {
				return nil, Usage("Use the collection identity and audience revision you reviewed")
			}
			query := url.Values{"collection_id": {args[1]}, "collection_revision": {args[2]}}
			if len(args) == 6 {
				identity, err := c.Identity(ctx)
				if err != nil {
					return nil, err
				}
				capability := "document-ocr-language-selection-v1"
				if strings.Contains(args[5], "+") {
					capability = "document-ocr-mixed-languages-v1"
				}
				if !identity.Supports(capability) {
					return nil, Usage("Upgrade the service before selecting an OCR language")
				}
				query.Set("ocr_language", args[5])
			}
			return uploadKnowledgeFile(ctx, c, args[3], "/v1/collections/imports?"+query.Encode(), args[1], revision)
		}
	case "collections":
		if len(args) == 1 {
			return c.Collections(ctx)
		}
	case "collection":
		if len(args) == 2 {
			return c.Collection(ctx, args[1])
		}
	case "create-collection":
		if len(args) == 2 {
			return c.CreateCollection(ctx, args[1])
		}
	case "update-collection":
		if len(args) == 3 {
			var body rag.CollectionUpdate
			if err := readCollectionReview(args[2], &body); err != nil {
				return nil, err
			}
			return c.UpdateCollection(ctx, args[1], body)
		}
	case "document-collection":
		if len(args) == 2 {
			return c.DocumentCollection(ctx, args[1])
		}
	case "move-document":
		if len(args) == 2 {
			var body rag.CollectionMove
			if err := readCollectionReview(args[1], &body); err != nil {
				return nil, err
			}
			return c.MoveDocument(ctx, body)
		}
	}
	return nil, Usage("Usage: offgrid kb collections|collection ID|collection-audit ID [--after N]|create-collection NAME|update-collection ID REVIEW.json|document-collection ID|move-document REVIEW.json|import-collection ID REVISION PATH [--ocr-language LANGUAGE]")
}
