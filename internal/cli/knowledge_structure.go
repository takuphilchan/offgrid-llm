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

func runKnowledgeStructure(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	if args[0] == "correct" {
		if len(args) != 2 {
			return nil, Usage("Usage: offgrid kb correct REVIEW.json")
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
		if !info.Mode().IsRegular() || info.Size() > 2<<20 {
			return nil, Usage("Choose a regular correction JSON file no larger than 2 MiB")
		}
		data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
		if err != nil {
			return nil, err
		}
		if len(data) > 2<<20 {
			return nil, Usage("Correction file exceeds 2 MiB")
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var request document.CorrectionRequest
		if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
			return nil, Usage("Provide document_id, expected_revision and edits in the correction file")
		}
		return client.CorrectDocument(ctx, request)
	}
	const usage = "offgrid kb structure ID [--revision REVISION] [--unit UNIT] [--offset N] [--limit N] [--relationship-offset N] [--navigation-offset N]"
	if len(args) < 2 || len(args)%2 != 0 {
		return nil, Usage("Usage: %s", usage)
	}
	values := map[string]string{}
	for i := 2; i < len(args); i += 2 {
		key := args[i]
		switch key {
		case "--revision", "--unit", "--offset", "--limit", "--relationship-offset", "--navigation-offset":
		default:
			return nil, Usage("Usage: %s", usage)
		}
		if _, ok := values[key]; ok {
			return nil, Usage("Do not repeat structure options")
		}
		values[key] = args[i+1]
	}
	offset, limit, links, navigation := 0, 100, 0, 0
	for key, target := range map[string]*int{"--offset": &offset, "--limit": &limit, "--relationship-offset": &links, "--navigation-offset": &navigation} {
		if value, ok := values[key]; ok {
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, Usage("Usage: %s", usage)
			}
			*target = n
		}
	}
	return client.DocumentStructure(ctx, args[1], values["--revision"], values["--unit"], offset, limit, links, navigation)
}
