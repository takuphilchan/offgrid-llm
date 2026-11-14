package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func inspectSavedResult(ctx context.Context, client *serviceclient.Client, reference string, out io.Writer, jsonMode bool) error {
	if reference == "" || strings.ContainsAny(reference, " \r\n\t") {
		return Usage("result RESULT_REFERENCE")
	}
	for offset := 0; ; {
		page, err := client.ResultPage(ctx, reference, offset, 16384)
		if err != nil {
			return err
		}
		if page.ID != reference || page.Offset != offset || page.NextOffset != offset+len(page.Content) || page.NextOffset > page.TotalBytes || !page.Complete && page.NextOffset <= offset || page.Complete && page.NextOffset != page.TotalBytes {
			return fmt.Errorf("invalid saved result page")
		}
		if jsonMode {
			err = json.NewEncoder(out).Encode(page)
		} else {
			_, err = io.WriteString(out, TerminalSafe(page.Content))
		}
		if err != nil {
			return err
		}
		if page.Complete {
			if !jsonMode {
				_, err = fmt.Fprintln(out)
			}
			return err
		}
		offset = page.NextOffset
	}
}
