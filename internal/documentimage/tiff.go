// Package documentimage decodes bounded document rasters without changing originals.
package documentimage

import (
	"context"
	"encoding/binary"
	"errors"
	"image"
	"io"

	"golang.org/x/image/tiff"
)

const TIFFRevision = "offgrid-tiff-v1-ximage-0.45.0"
const MaxBytes = 24 << 20
const MaxPages = 100

var ErrInvalid = errors.New("invalid TIFF document")
var ErrLimit = errors.New("TIFF document exceeds supported limits")
var ErrUnsupported = errors.New("unsupported TIFF document structure")

type Page struct {
	Offset                     uint32
	Width, Height, Orientation int
}

// Inspect traverses the complete main IFD chain before decoding any pixels.
// SubIFDs can hide other image planes; reject them rather than silently dropping
// them. BigTIFF and legacy JPEG-in-TIFF are not supported by this adapter.
func Inspect(ctx context.Context, data []byte) ([]Page, error) {
	if len(data) > MaxBytes {
		return nil, ErrLimit
	}
	order, err := byteOrder(data)
	if err != nil {
		return nil, err
	}
	offset := order.Uint32(data[4:8])
	seen := map[uint32]bool{}
	pages := []Page{}
	var pixels int64
	for offset != 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[offset] {
			return nil, ErrInvalid
		}
		seen[offset] = true
		if len(pages) >= MaxPages {
			return nil, ErrLimit
		}
		if offset < 8 || uint64(offset)+2 > uint64(len(data)) {
			return nil, ErrInvalid
		}
		n := int(order.Uint16(data[offset : offset+2]))
		end := uint64(offset) + 2 + uint64(n)*12
		if n < 1 || n > 256 {
			return nil, ErrLimit
		}
		if end+4 > uint64(len(data)) {
			return nil, ErrInvalid
		}
		page := Page{Offset: offset, Orientation: 1}
		tileWidth, tileHeight := 0, 0
		tags := map[uint16]bool{}
		for pos := int(offset) + 2; pos < int(end); pos += 12 {
			e := data[pos : pos+12]
			tag := order.Uint16(e)
			typ := order.Uint16(e[2:])
			count := order.Uint32(e[4:])
			if tags[tag] {
				return nil, ErrInvalid
			}
			tags[tag] = true
			width := uint64(0)
			switch typ {
			case 1, 2, 6, 7:
				width = 1
			case 3, 8:
				width = 2
			case 4, 9, 11:
				width = 4
			case 5, 10, 12:
				width = 8
			default:
				return nil, ErrUnsupported
			}
			size := width * uint64(count)
			if size > 1<<20 {
				return nil, ErrLimit
			}
			value := e[8:12]
			if size > 4 {
				start := uint64(order.Uint32(value))
				if start < 8 || start+size > uint64(len(data)) {
					return nil, ErrInvalid
				}
				value = data[start : start+size]
			}
			if tag == 330 && count > 0 {
				return nil, ErrUnsupported
			}
			switch tag {
			case 256, 257, 274, 322, 323, 278, 259, 284:
				if count != 1 || (typ != 3 && typ != 4) {
					return nil, ErrInvalid
				}
				v := uint32(order.Uint16(value))
				if typ == 4 {
					v = order.Uint32(value)
				}
				switch tag {
				case 256:
					page.Width = int(v)
				case 257:
					page.Height = int(v)
				case 274:
					if v < 1 || v > 8 {
						return nil, ErrInvalid
					}
					page.Orientation = int(v)
				case 322, 323:
					if v < 1 || v > 8192 {
						return nil, ErrLimit
					}
					if tag == 322 {
						tileWidth = int(v)
					} else {
						tileHeight = int(v)
					}
				case 278:
					if v == 0 {
						return nil, ErrInvalid
					}
				case 259:
					switch v {
					case 1, 3, 4, 5, 8, 32773, 32946:
					default:
						return nil, ErrUnsupported
					}
				case 284:
					if v != 1 {
						return nil, ErrUnsupported
					}
				}
			}
		}
		if page.Width < 1 || page.Height < 1 {
			return nil, ErrInvalid
		}
		if int64(tileWidth)*int64(tileHeight) > 16000000 {
			return nil, ErrLimit
		}
		area := int64(page.Width) * int64(page.Height)
		if page.Width > 8192 || page.Height > 8192 || area > 16000000 {
			return nil, ErrLimit
		}
		pixels += area
		if pixels > 200000000 {
			return nil, ErrLimit
		}
		pages = append(pages, page)
		offset = order.Uint32(data[end : end+4])
	}
	if len(pages) == 0 {
		return nil, ErrInvalid
	}
	return pages, nil
}

func byteOrder(data []byte) (binary.ByteOrder, error) {
	if len(data) < 8 {
		return nil, ErrInvalid
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, ErrInvalid
	}
	if order.Uint16(data[2:4]) == 43 {
		return nil, ErrUnsupported
	}
	if order.Uint16(data[2:4]) != 42 {
		return nil, ErrInvalid
	}
	return order, nil
}

// pageReader redirects only the first-IFD pointer in a read-only view. Every
// data offset still addresses the original file; no rebasing or source mutation.
// A byte-read ceiling also bounds pathological repeated strip/tile references.
type pageReader struct {
	ctx       context.Context
	data      []byte
	header    [8]byte
	position  int64
	remaining int64
}

func (r *pageReader) ReadAt(p []byte, off int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if off < 0 || off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		return 0, ErrLimit
	}
	r.remaining -= int64(len(p))
	n := copy(p, r.data[off:])
	for i := 0; i < n && off+int64(i) < 8; i++ {
		p[i] = r.header[off+int64(i)]
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
func (r *pageReader) Read(p []byte) (int, error) {
	n, e := r.ReadAt(p, r.position)
	r.position += int64(n)
	return n, e
}

// Decode accepts only a descriptor returned by Inspect for this exact source.
// Callers retain the inspected set and process one page at a time.
func Decode(ctx context.Context, data []byte, page Page) (image.Image, error) {
	pages, err := Inspect(ctx, data)
	if err != nil {
		return nil, err
	}
	found := false
	for _, p := range pages {
		if p == page {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrInvalid
	}
	order, _ := byteOrder(data)
	r := &pageReader{ctx: ctx, data: data, remaining: 128 << 20}
	copy(r.header[:], data[:8])
	order.PutUint32(r.header[4:], page.Offset)
	config, err := tiff.DecodeConfig(r)
	if err != nil {
		return nil, err
	}
	if config.Width != page.Width || config.Height != page.Height {
		return nil, ErrInvalid
	}
	r.position = 0
	pixels, err := tiff.Decode(r)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if pixels.Bounds().Dx() != page.Width || pixels.Bounds().Dy() != page.Height {
		return nil, ErrInvalid
	}
	return pixels, nil
}
