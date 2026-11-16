package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/png"

	"github.com/takuphilchan/offgrid-llm/internal/imageinput"
)

const maxOrientedImageBytes = 16 << 20
const maxPreparedImageBytes = 32 << 20

// PrepareChatImages returns an inference-only projection. Saved messages and
// uploaded originals retain their exact bytes and durable request identity.
// Non-default Exif orientations become lossless PNG pixels with no orientation
// tag, so a runtime cannot ignore or apply the transform twice. Counting and
// generation must use this same projection. No resize or lossy JPEG re-encoding.
func PrepareChatImages(messages []ChatMessage) ([]ChatMessage, error) {
	out := append([]ChatMessage(nil), messages...)
	budget := maxPreparedImageBytes
	for i, message := range messages {
		if _, ok := message.Content.(string); ok || message.Content == nil {
			continue
		}
		raw, err := json.Marshal(message.Content)
		if err != nil {
			return nil, ErrInvalidMessageContent
		}
		var parts []ChatContentPart
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&parts) != nil || len(parts) > maxContentParts {
			return nil, ErrInvalidMessageContent
		}
		for j := range parts {
			if parts[j].Type != "image_url" {
				continue
			}
			if parts[j].ImageURL == nil || message.Role != "user" {
				return nil, ErrInvalidMessageContent
			}
			url, size, err := prepareDataImage(*parts[j].ImageURL)
			if err != nil || size > budget {
				return nil, ErrInvalidMessageContent
			}
			budget -= size
			parts[j].ImageURL.URL = url
		}
		out[i].Content = parts
	}
	return out, nil
}

func prepareDataImage(input ChatImageURL) (string, int, error) {
	data, pixels, orientation, err := decodeDataImage(input, maxImageBytes)
	if err != nil {
		return "", 0, err
	}
	if orientation == 1 {
		return input.URL, len(data), nil
	}
	pixels, err = imageinput.Orient(pixels, orientation)
	if err != nil {
		return "", 0, ErrInvalidMessageContent
	}
	var encodedPixels imageOutput
	if err := png.Encode(&encodedPixels, pixels); err != nil {
		return "", 0, ErrInvalidMessageContent
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encodedPixels.Bytes()), encodedPixels.Len(), nil
}

type imageOutput struct{ bytes.Buffer }

func (b *imageOutput) Write(value []byte) (int, error) {
	if len(value) > maxOrientedImageBytes-b.Len() {
		return 0, ErrInvalidMessageContent
	}
	return b.Buffer.Write(value)
}
