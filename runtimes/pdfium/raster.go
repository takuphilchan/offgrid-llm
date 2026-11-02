package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
)

const rasterRevision = revision + "-raster2"
const rasterDPI = 200
const maxRasterPixels = 16_000_000
const maxRasterPNG = 24 << 20

type rasterReceipt struct {
	Protocol int            `json:"protocol"`
	Revision string         `json:"revision"`
	SHA256   string         `json:"sha256"`
	Page     int            `json:"page"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	PNG      []byte         `json:"png,omitempty"`
	Native   []nativeRegion `json:"native,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func rasterDimensions(width, height float64) (int, int, bool) {
	w, h := math.Ceil(width*rasterDPI/72), math.Ceil(height*rasterDPI/72)
	if math.IsNaN(w) || math.IsNaN(h) || math.IsInf(w, 0) || math.IsInf(h, 0) || w < 1 || h < 1 || w > 8192 || h > 8192 || w*h > maxRasterPixels {
		return 0, 0, false
	}
	return int(w), int(h), true
}

type boundedPNG struct{ bytes.Buffer }

func (b *boundedPNG) Write(data []byte) (int, error) {
	if b.Len()+len(data) > maxRasterPNG {
		return 0, fmt.Errorf("raster output limit")
	}
	return b.Buffer.Write(data)
}

// The module still has no filesystem, environment or network access. Raster
// output leaves only through this bounded receipt, tied to the source digest.
func renderPDFPage(ctx context.Context, data []byte, number int, native ...*[]nativeRegion) ([]byte, int, int, string) {
	if number < 1 || number > maxPages {
		return nil, 0, 0, "document_invalid"
	}
	d, code := openPDF(ctx, data)
	if code != "" {
		return nil, 0, 0, code
	}
	defer d.close()
	if uint64(number) > d.pages {
		return nil, 0, 0, "document_invalid"
	}
	e := d.engine
	page, err := e.call("FPDF_LoadPage", d.handle, uint64(number-1))
	if err != nil || page == 0 {
		return nil, 0, 0, "document_invalid"
	}
	defer e.call("FPDF_ClosePage", page)
	w, errW := e.call("FPDF_GetPageWidthF", page)
	h, errH := e.call("FPDF_GetPageHeightF", page)
	width, height, ok := rasterDimensions(float64(math.Float32frombits(uint32(w))), float64(math.Float32frombits(uint32(h))))
	if errW != nil || errH != nil || !ok {
		return nil, 0, 0, "document_limit"
	}
	if len(native) != 0 {
		regions, code := nativeRegions(e, page, width, height)
		if code != "" {
			return nil, 0, 0, code
		}
		*native[0] = regions
	}
	bitmap, err := e.call("FPDFBitmap_Create", uint64(width), uint64(height), 1)
	if err != nil || bitmap == 0 {
		return nil, 0, 0, "document_limit"
	}
	defer e.call("FPDFBitmap_Destroy", bitmap)
	if _, err = e.call("FPDFBitmap_FillRect", bitmap, 0, 0, uint64(width), uint64(height), 0xffffffff); err != nil {
		return nil, 0, 0, "document_invalid"
	}
	// No annotations or interactive forms; document content only. Intrinsic PDF
	// rotation is honored by the page API; additional rotation is zero.
	if _, err = e.call("FPDF_RenderPageBitmap", bitmap, page, 0, 0, uint64(width), uint64(height), 0, 0); err != nil {
		return nil, 0, 0, "document_invalid"
	}
	ptr, errP := e.call("FPDFBitmap_GetBuffer", bitmap)
	stride, errS := e.call("FPDFBitmap_GetStride", bitmap)
	if errP != nil || errS != nil || stride != uint64(width*4) {
		return nil, 0, 0, "document_invalid"
	}
	pixels, ok := e.module.Memory().Read(uint32(ptr), uint32(stride*uint64(height)))
	if !ok {
		return nil, 0, 0, "document_invalid"
	}
	// PDFium uses BGRA. The white composited surface is opaque; convert to RGBA.
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = pixels[i+2], pixels[i+1], pixels[i], 255
	}
	if ctx.Err() != nil {
		return nil, 0, 0, "document_timeout"
	}
	var encoded boundedPNG
	if err = png.Encode(&encoded, out); err != nil {
		return nil, 0, 0, "document_limit"
	}
	return encoded.Bytes(), width, height, ""
}
