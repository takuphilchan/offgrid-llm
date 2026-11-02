// Package documentocr validates OCR evidence shared by the isolated worker and
// service. It does not perform inference, grant access or interpret instructions.
package documentocr

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OCR words retain geometry and confidence for source attribution and mixed-page
// reconciliation. Reading these words is not permission to declare a page ready.
type Word struct {
	Text       string
	Bounds     [4]int
	Confidence int
	Line       int
}

// parseHOCR accepts one bounded Tesseract hOCR page, not HTML for display. XML
// decoding never fetches its DTD or external entities. Unknown entities fail.
func ParseHOCR(content string, width, height int) ([]Word, error) {
	invalid := func() error { return fmt.Errorf("invalid OCR page receipt") }
	if len(content) == 0 || len(content) > 2<<20 || !utf8.ValidString(content) || width < 1 || height < 1 || width > 8192 || height > 8192 || int64(width)*int64(height) > 16_000_000 {
		return nil, invalid()
	}
	decoder := xml.NewDecoder(strings.NewReader(content))
	var words []Word
	var text strings.Builder
	var current Word
	depth, pageDepth, lineDepth, wordDepth, pages, lines, textBytes, roots := 0, 0, 0, 0, 0, 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, invalid()
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				roots++
				if roots != 1 {
					return nil, invalid()
				}
			}
			if depth > 64 {
				return nil, invalid()
			}
			class, title := "", ""
			for _, attr := range value.Attr {
				switch attr.Name.Local {
				case "class":
					class = attr.Value
				case "title":
					title = attr.Value
				}
			}
			has := func(name string) bool {
				for _, c := range strings.Fields(class) {
					if c == name {
						return true
					}
				}
				return false
			}
			switch {
			case has("ocr_page"):
				pages++
				if pages != 1 || pageDepth != 0 || wordDepth != 0 {
					return nil, invalid()
				}
				bounds, _, err := hocrGeometry(title, width, height, false)
				if err != nil || bounds != [4]int{0, 0, width, height} {
					return nil, invalid()
				}
				pageDepth = depth
			case has("ocr_line"):
				if pageDepth == 0 || lineDepth != 0 || wordDepth != 0 {
					return nil, invalid()
				}
				lineDepth = depth
				lines++
			case has("ocrx_word"):
				if pageDepth == 0 || lineDepth == 0 || wordDepth != 0 || len(words) >= 20000 {
					return nil, invalid()
				}
				bounds, confidence, err := hocrGeometry(title, width, height, true)
				if err != nil {
					return nil, invalid()
				}
				current = Word{Bounds: bounds, Confidence: confidence, Line: lines}
				wordDepth = depth
				text.Reset()
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return nil, invalid()
			}
			if wordDepth != 0 {
				textBytes += len(value)
				if textBytes > 1<<20 {
					return nil, invalid()
				}
				text.Write(value)
			}
		case xml.EndElement:
			if depth == wordDepth {
				current.Text = strings.TrimSpace(text.String())
				if current.Text == "" {
					return nil, invalid()
				}
				words = append(words, current)
				wordDepth = 0
			}
			if depth == lineDepth {
				lineDepth = 0
			}
			if depth == pageDepth {
				pageDepth = 0
			}
			depth--
		}
	}
	if depth != 0 || pages != 1 || wordDepth != 0 {
		return nil, invalid()
	}
	return words, nil
}

func hocrGeometry(title string, width, height int, requireConfidence bool) ([4]int, int, error) {
	var bounds [4]int
	confidence, foundBounds, foundConfidence := 0, false, false
	invalid := func() ([4]int, int, error) { return bounds, 0, fmt.Errorf("invalid OCR geometry") }
	for _, property := range strings.Split(title, ";") {
		fields := strings.Fields(property)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "bbox":
			if foundBounds || len(fields) != 5 {
				return invalid()
			}
			foundBounds = true
			for i := range bounds {
				n, err := strconv.Atoi(fields[i+1])
				if err != nil {
					return invalid()
				}
				bounds[i] = n
			}
		case "x_wconf":
			if foundConfidence || len(fields) != 2 {
				return invalid()
			}
			foundConfidence = true
			n, err := strconv.Atoi(fields[1])
			if err != nil || n < 0 || n > 100 {
				return invalid()
			}
			confidence = n
		}
	}
	if !foundBounds || requireConfidence && !foundConfidence || bounds[0] < 0 || bounds[1] < 0 || bounds[2] <= bounds[0] || bounds[3] <= bounds[1] || bounds[2] > width || bounds[3] > height {
		return invalid()
	}
	return bounds, confidence, nil
}
