package cli

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Exact block text, offsets and relationships remain structured JSON in both
// terminal and pipe use. Inspection never publishes a private file to Knowledge.
func runSessionStructure(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer) error {
	usage := func() error {
		return Usage("session attachment-structure NAME ID REVISION [BLOCK_OFFSET] [RELATIONSHIP_OFFSET] [NAVIGATION_OFFSET]")
	}
	if len(args) < 4 || len(args) > 7 {
		return usage()
	}
	offsets := []int{0, 0, 0}
	for i, value := range args[4:] {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return usage()
		}
		offsets[i] = n
	}
	page, err := client.SessionAttachmentStructure(ctx, args[1], args[2], args[3], "", offsets[0], 100, offsets[1], offsets[2])
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(page)
}
