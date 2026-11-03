package serviceclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

var sessionFileID = regexp.MustCompile(`^file-[a-f0-9]{32}$`)

func validAttachmentID(id string) bool { return sessionFileID.MatchString(id) }
func sessionFilesPath(name string) string {
	return "/v1/sessions/" + url.PathEscape(name) + "/attachments"
}

// UploadSessionFile snapshots explicitly supplied bytes, uses only the selected
// authenticated service, and never retries a mutation after an uncertain reply.
func (c *Client) UploadSessionFile(ctx context.Context, name, filename string, data []byte) (sessions.Attachment, error) {
	if name == "" || filename == "" || len(filename) > 255 || strings.ContainsAny(filename, `/\`) || strings.IndexFunc(filename, unicode.IsControl) >= 0 || len(data) == 0 || int64(len(data)) > sessions.MaxAttachmentBytes {
		return sessions.Attachment{}, &Error{Code: "attachment_limit", Message: "Choose a plain filename and a nonempty file of at most 50 MiB."}
	}
	var envelope bytes.Buffer
	form := multipart.NewWriter(&envelope)
	if _, err := form.CreateFormFile("file", filename); err != nil {
		return sessions.Attachment{}, err
	}
	header := append([]byte(nil), envelope.Bytes()...)
	envelope.Reset()
	if err := form.Close(); err != nil {
		return sessions.Attachment{}, err
	}
	// Avoid copying up to 50 MiB into a second multipart buffer.
	body := io.MultiReader(bytes.NewReader(header), bytes.NewReader(data), bytes.NewReader(envelope.Bytes()))
	client := *c
	transport := *c.http
	transport.Timeout = 2 * time.Minute
	client.http = &transport
	response, err := client.Do(ctx, http.MethodPost, sessionFilesPath(name), form.FormDataContentType(), body)
	if err != nil {
		return sessions.Attachment{}, err
	}
	defer response.Body.Close()
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if err := DecodeContext(ctx, response.Body, &result); err != nil {
		return sessions.Attachment{}, err
	}
	digest := sha256.Sum256(data)
	a := result.Attachment
	_, revisionErr := hex.DecodeString(a.Revision)
	if !validAttachmentID(a.ID) || a.Name != filename || a.Bytes != int64(len(data)) || a.SHA256 != hex.EncodeToString(digest[:]) || a.Status != "ready" || len(a.Revision) != 64 || revisionErr != nil {
		return sessions.Attachment{}, &Error{Code: "invalid_response", Message: "The uploaded source did not return matching identity and readiness. Inspect saved attachments before retrying."}
	}
	return a, nil
}

func (c *Client) SessionFiles(ctx context.Context, name string) ([]sessions.Attachment, error) {
	var result struct {
		Attachments []sessions.Attachment `json:"attachments"`
	}
	if err := c.JSON(ctx, http.MethodGet, sessionFilesPath(name), nil, &result); err != nil {
		return nil, err
	}
	if result.Attachments == nil {
		return nil, &Error{Code: "invalid_response", Message: "The service returned no attachment inventory."}
	}
	for _, a := range result.Attachments {
		if !validAttachmentID(a.ID) || a.Name == "" || a.Bytes < 1 || a.Bytes > sessions.MaxAttachmentBytes {
			return nil, &Error{Code: "invalid_response", Message: "The service returned an invalid attachment identity."}
		}
	}
	return result.Attachments, nil
}

func (c *Client) RemoveSessionFile(ctx context.Context, name, id string) error {
	if !validAttachmentID(id) {
		return fmt.Errorf("invalid attachment ID")
	}
	var result struct {
		Removed bool `json:"removed"`
	}
	if err := c.JSON(ctx, http.MethodDelete, sessionFilesPath(name)+"/"+id, nil, &result); err != nil {
		return err
	}
	if !result.Removed {
		return &Error{Code: "invalid_response", Message: "The service did not confirm source removal."}
	}
	return nil
}

// PrepareSessionFile retries extraction of a retained original, never its upload.
// The response must preserve the source identity inspected by this caller.
func (c *Client) PrepareSessionFile(ctx context.Context, name string, source sessions.Attachment) (sessions.Attachment, error) {
	return c.prepareSessionFile(ctx, name, source, false)
}

// Explicit caller choice; never silently accepts partial extraction on retry.
func (c *Client) PrepareSessionFileTextOnly(ctx context.Context, name string, source sessions.Attachment) (sessions.Attachment, error) {
	return c.prepareSessionFile(ctx, name, source, true)
}

func (c *Client) prepareSessionFile(ctx context.Context, name string, source sessions.Attachment, textOnly bool) (sessions.Attachment, error) {
	if name == "" || !validAttachmentID(source.ID) {
		return sessions.Attachment{}, fmt.Errorf("invalid attachment identity")
	}
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := *c
	transport := *c.http
	transport.Timeout = 2 * time.Minute
	client.http = &transport
	path := sessionFilesPath(name) + "/" + source.ID + "/prepare"
	if textOnly {
		path += "?text_only=true"
	}
	if err := client.JSON(bounded, http.MethodPost, path, struct{}{}, &result); err != nil {
		return sessions.Attachment{}, err
	}
	a := result.Attachment
	_, revisionErr := hex.DecodeString(a.Revision)
	if a.ID != source.ID || a.SHA256 != source.SHA256 || a.Name != source.Name || a.Bytes != source.Bytes || a.Status != "ready" || len(a.Revision) != 64 || revisionErr != nil {
		return sessions.Attachment{}, &Error{Code: "invalid_response", Message: "Preparation did not confirm the retained source identity. Inspect saved attachments before retrying."}
	}
	return a, nil
}
