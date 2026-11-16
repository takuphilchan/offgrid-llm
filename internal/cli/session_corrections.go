package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func runSessionCorrections(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer) error {
	if len(args) == 3 && args[0] == "attachment-revisions" {
		result, err := client.SessionAttachmentRevisions(ctx, args[1], args[2])
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(result)
	}
	if len(args) != 4 || args[0] != "correct-attachment" {
		return Usage("session attachment-revisions NAME ID | correct-attachment NAME ID REVIEW.json")
	}
	f, err := os.Open(args[3])
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return Usage("Choose a regular correction JSON file no larger than 2 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 2<<20 {
		return Usage("Correction file exceeds 2 MiB")
	}
	var body serviceclient.AttachmentCorrectionRequest
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF {
		return Usage("Provide expected_revision, extraction_revision and edits in the correction file")
	}
	result, err := client.CorrectSessionAttachment(ctx, args[1], args[2], body)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}
