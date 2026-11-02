// offgrid-document-worker accepts one PDF on stdin and emits one versioned
// receipt. It has no path, shell, URL, script, password or credential interface.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const revision = "pdfium-8057-0001cb839e62425b-offgrid1"
const maxInput = 50 << 20
const maxText = 16 << 20
const maxPages = 1000

type pageUnit struct {
	Page  int    `json:"page"`
	Text  string `json:"text"`
	State string `json:"state"`
}

type receipt struct {
	Protocol int        `json:"protocol"`
	Revision string     `json:"revision"`
	SHA256   string     `json:"sha256"`
	Pages    []pageUnit `json:"pages,omitempty"`
	Error    string     `json:"error,omitempty"`
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--licenses" {
		_, _ = io.WriteString(os.Stdout, engineNotices)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	// Parent also supervises stdin delivery and hard process termination.
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxInput+1))
	digest := sha256.Sum256(data)
	if len(os.Args) != 1 {
		page := 0
		if len(os.Args) == 3 && os.Args[1] == "--raster-page" {
			page, _ = strconv.Atoi(os.Args[2])
		}
		rendered := rasterReceipt{Protocol: 1, Revision: rasterRevision, SHA256: hex.EncodeToString(digest[:]), Page: page}
		if err != nil {
			rendered.Error = "document_invalid"
		} else if page < 1 || page > maxPages {
			rendered.Error = "document_invalid"
		} else {
			rendered.PNG, rendered.Width, rendered.Height, rendered.Error = renderPDFPage(ctx, data, page, &rendered.Native)
		}
		if ctx.Err() != nil {
			rendered.PNG = nil
			rendered.Width, rendered.Height = 0, 0
			rendered.Error = "document_timeout"
		}
		if rendered.Error != "" {
			rendered.Native = nil
		}
		if err := json.NewEncoder(os.Stdout).Encode(rendered); err != nil {
			os.Exit(1)
		}
		return
	}
	r := receipt{Protocol: 1, Revision: revision, SHA256: hex.EncodeToString(digest[:])}
	if err != nil {
		r.Error = "document_invalid"
	} else if len(data) > maxInput {
		r.Error = "document_limit"
	} else {
		r.Pages, r.Error = extract(ctx, data)
	}
	if ctx.Err() != nil {
		r.Pages = nil
		r.Error = "document_timeout"
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		os.Exit(1)
	}
}

func extract(ctx context.Context, data []byte) ([]pageUnit, string) {
	document, code := openPDF(ctx, data)
	if code != "" {
		return nil, code
	}
	defer document.close()
	engine, doc, count := document.engine, document.handle, document.pages
	pages := make([]pageUnit, 0, count)
	total := 0
	for i := uint64(0); i < count; i++ {
		if ctx.Err() != nil {
			return nil, "document_timeout"
		}
		page, err := engine.call("FPDF_LoadPage", doc, i)
		if err != nil || page == 0 {
			return nil, "document_invalid"
		}
		text, needsOCR, code := readPage(engine, page, maxText-total)
		_, closeErr := engine.call("FPDF_ClosePage", page)
		if code != "" {
			return nil, code
		}
		if closeErr != nil {
			return nil, "document_invalid"
		}
		total += len(text)
		state := "ready"
		if needsOCR {
			state = "ocr_required"
		} else if strings.TrimSpace(text) == "" {
			state = "empty"
		}
		pages = append(pages, pageUnit{Page: int(i + 1), Text: text, State: state})
	}
	return pages, ""
}

func pageNeedsOCR(engine *pdfEngine, page uint64, text string) (bool, string) {
	objects, err := engine.call("FPDFPage_CountObjects", page)
	if err != nil {
		return false, "document_invalid"
	}
	if objects > 10000 {
		return false, "document_limit"
	}
	if strings.TrimSpace(text) == "" {
		return objects > 0, ""
	}
	for i := uint64(0); i < objects; i++ {
		object, err := engine.call("FPDFPage_GetObject", page, i)
		if err != nil {
			return false, "document_invalid"
		}
		kind, err := engine.call("FPDFPageObj_GetType", object)
		if err != nil {
			return false, "document_invalid"
		}
		// Forms can contain raster content. Until recursively qualified, preserve
		// native text but do not present the complete page as fully extracted.
		if kind == 3 || kind == 5 { // FPDF_PAGEOBJ_IMAGE / FPDF_PAGEOBJ_FORM
			return true, ""
		}
	}
	return false, ""
}
