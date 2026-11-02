package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func runKnowledgeOCR(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	path := "/v1/documents/ocr-languages"
	var result map[string]any
	if len(args) == 1 && args[0] == "ocr-languages" {
		if err := client.JSON(ctx, http.MethodGet, path, nil, &result); err != nil {
			return nil, err
		}
	} else if len(args) == 3 && args[0] == "import-ocr-language" {
		info, err := os.Lstat(args[2])
		if err != nil {
			return nil, err
		}
		const limit = 32 << 20
		if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
			return nil, Usage("Choose a regular OCR language file up to 32 MiB")
		}
		file, err := os.Open(args[2])
		if err != nil {
			return nil, err
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !os.SameFile(info, opened) {
			return nil, Usage("OCR language file changed before upload")
		}
		data, err := io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil {
			return nil, err
		}
		if len(data) > limit {
			return nil, Usage("OCR language file exceeds 32 MiB")
		}
		response, err := client.Do(ctx, http.MethodPost, path+"?language="+url.QueryEscape(args[1]), "application/octet-stream", bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if err = serviceclient.DecodeContext(ctx, response.Body, &result); err != nil {
			return nil, err
		}
	} else {
		return nil, Usage("Usage: offgrid kb ocr-languages | import-ocr-language LANGUAGE PATH")
	}
	rows, ok := result["languages"].([]any)
	valid := ok && len(args) == 1
	for _, row := range rows {
		if item, ok := row.(map[string]any); ok && len(args) == 3 && item["language"] == args[1] && item["state"] == "installed_verified" {
			valid = true
		}
	}
	if !valid {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "OCR setup was not confirmed. Check language status before retrying."}
	}
	return result, nil
}
