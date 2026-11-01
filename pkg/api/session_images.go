package api

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"strings"
	"unicode/utf8"
)

const MaxSessionImages = 4
const MaxSessionImageBytes = 2 << 20
const MaxSessionImageRequestBytes = 12 << 20

// SessionImage is saved inside the scoped conversation, never fetched by URL.
type SessionImage struct {
	Name    string `json:"name"`
	DataURL string `json:"data_url"`
}

var ErrSessionImages = errors.New("attach up to four valid PNG/JPEG images, at most 2 MiB each and 8192 pixels per edge")

func ValidateSessionImages(images []SessionImage) error {
	if len(images) > MaxSessionImages {
		return ErrSessionImages
	}
	for _, item := range images {
		if len(item.Name) == 0 || len(item.Name) > 200 || !utf8.ValidString(item.Name) || strings.ContainsAny(item.Name, "\x00\r\n") {
			return ErrSessionImages
		}
		if len(item.DataURL) > base64.StdEncoding.EncodedLen(MaxSessionImageBytes)+32 {
			return ErrSessionImages
		}
		if err := validateDataImage(ChatImageURL{URL: item.DataURL}); err != nil {
			return ErrSessionImages
		}
		_, encoded, _ := strings.Cut(item.DataURL, ",")
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(data) > MaxSessionImageBytes {
			return ErrSessionImages
		}
		// DecodeConfig alone accepts truncated pixel data. Dimensions were bounded above.
		if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			return ErrSessionImages
		}
	}
	return nil
}

func SessionContent(text string, images []SessionImage) any {
	if len(images) == 0 {
		return text
	}
	parts := []ChatContentPart{{Type: "text", Text: text}}
	for _, item := range images {
		parts = append(parts, ChatContentPart{Type: "image_url", ImageURL: &ChatImageURL{URL: item.DataURL}})
	}
	return parts
}
