package models

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type resumableResponse struct {
	response *http.Response
	offset   int64
	total    int64
	complete bool
}

// openResumableResponse opens a download at the current temporary-file offset.
// The remote response is authoritative: stale catalog sizes and servers that
// ignore Range must never cause data to be appended twice.
func openResumableResponse(client *http.Client, sourceURL, tmpPath, userAgent string) (*resumableResponse, error) {
	return openResumableResponseContext(context.Background(), client, sourceURL, tmpPath, userAgent)
}

func openResumableResponseContext(ctx context.Context, client *http.Client, sourceURL, tmpPath, userAgent string) (*resumableResponse, error) {
	var offset int64
	if stat, err := os.Stat(tmpPath); err == nil {
		offset = stat.Size()
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect partial download: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create download request: %w", err)
		}
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0 {
			remoteTotal, ok := parseUnsatisfiedContentRange(resp.Header.Get("Content-Range"))
			resp.Body.Close()
			if ok && offset == remoteTotal {
				return &resumableResponse{offset: offset, total: remoteTotal, complete: true}, nil
			}
			if err := os.Truncate(tmpPath, 0); err != nil {
				return nil, fmt.Errorf("reset invalid partial download: %w", err)
			}
			offset = 0
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		var total int64
		if resp.StatusCode == http.StatusPartialContent {
			start, remoteTotal, ok := parsePartialContentRange(resp.Header.Get("Content-Range"))
			if !ok || start != offset {
				resp.Body.Close()
				return nil, fmt.Errorf("invalid resume response: %q", resp.Header.Get("Content-Range"))
			}
			total = remoteTotal
		} else {
			// A 200 response to a ranged request contains the whole object. Reset
			// the partial file so the response replaces it instead of appending.
			if offset > 0 {
				if err := os.Truncate(tmpPath, 0); err != nil {
					resp.Body.Close()
					return nil, fmt.Errorf("reset unresumable partial download: %w", err)
				}
				offset = 0
			}
			if resp.ContentLength >= 0 {
				total = resp.ContentLength
			}
		}

		if total <= 0 && resp.ContentLength >= 0 {
			total = offset + resp.ContentLength
		}
		return &resumableResponse{response: resp, offset: offset, total: total}, nil
	}

	return nil, fmt.Errorf("download could not recover from an invalid partial file")
}

func parseUnsatisfiedContentRange(header string) (int64, bool) {
	const prefix = "bytes */"
	if !strings.HasPrefix(header, prefix) {
		return 0, false
	}
	total, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(header, prefix)), 10, 64)
	return total, err == nil && total >= 0
}

func parsePartialContentRange(header string) (start, total int64, ok bool) {
	if !strings.HasPrefix(header, "bytes ") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(header, "bytes "), "/")
	if len(parts) != 2 || parts[1] == "*" {
		return 0, 0, false
	}
	rangeParts := strings.Split(parts[0], "-")
	if len(rangeParts) != 2 {
		return 0, 0, false
	}
	parsedStart, errStart := strconv.ParseInt(rangeParts[0], 10, 64)
	parsedEnd, errEnd := strconv.ParseInt(rangeParts[1], 10, 64)
	parsedTotal, errTotal := strconv.ParseInt(parts[1], 10, 64)
	if errStart != nil || errEnd != nil || errTotal != nil || parsedStart < 0 || parsedEnd < parsedStart || parsedTotal <= parsedEnd {
		return 0, 0, false
	}
	return parsedStart, parsedTotal, true
}
