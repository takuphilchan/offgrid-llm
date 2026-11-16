package documentimage

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
	"image"
	"testing"
)

func TestTIFFPageIdentityAndByteOrder(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		a, b := image.NewGray(image.Rect(0, 0, 5, 3)), image.NewGray(image.Rect(0, 0, 7, 4))
		for i := range b.Pix {
			b.Pix[i] = 155
		}
		source := tifffixture.Encode([]image.Image{a, b}, []uint16{1, 6}, order)
		original := bytes.Clone(source)
		pages, err := Inspect(context.Background(), source)
		if err != nil || len(pages) != 2 || pages[1].Orientation != 6 {
			t.Fatal(pages, err)
		}
		for i, p := range pages {
			im, err := Decode(context.Background(), source, p)
			if err != nil {
				t.Fatal(err)
			}
			r, _, _, _ := im.At(0, 0).RGBA()
			if (i == 0 && r != 0) || (i == 1 && r != 155*257) {
				t.Fatal("wrong page", i, r)
			}
		}
		if !bytes.Equal(source, original) {
			t.Fatal("original mutated")
		}
		forged := pages[0]
		forged.Width++
		if _, err = Decode(context.Background(), source, forged); err == nil {
			t.Fatal("forged descriptor")
		}
	}
}

func TestTIFFRejectsDirectoryAndAllocationAttacks(t *testing.T) {
	base := tifffixture.TwoPages()
	order := binary.LittleEndian
	for name, mutate := range map[string]func([]byte){
		"cycle":               func(b []byte) { order.PutUint32(b[8+2+11*12:], 8) },
		"oversize":            func(b []byte) { order.PutUint32(b[8+2+8:], 8193) },
		"duplicate":           func(b []byte) { order.PutUint16(b[8+2+12:], 256) },
		"bad orientation":     func(b []byte) { order.PutUint16(b[8+2+6*12+8:], 9) },
		"subIFD":              func(b []byte) { order.PutUint16(b[8+2+10*12:], 330) },
		"bigTIFF":             func(b []byte) { order.PutUint16(b[2:], 43) },
		"bad offset":          func(b []byte) { order.PutUint32(b[4:], 0xffffffff) },
		"many entries":        func(b []byte) { order.PutUint16(b[8:], 65535) },
		"metadata allocation": func(b []byte) { order.PutUint32(b[8+2+4:], 0xffffffff) },
	} {
		t.Run(name, func(t *testing.T) {
			data := bytes.Clone(base)
			mutate(data)
			if _, err := Inspect(context.Background(), data); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, base); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	pages, err := Inspect(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	bad := bytes.Clone(base)
	order.PutUint32(bad[8+2+5*12+8:], 0xffffffff)
	if _, err := Decode(context.Background(), bad, pages[0]); err == nil {
		t.Fatal("bad strip accepted")
	}
}

func TestTIFFPageLimit(t *testing.T) {
	pages := make([]image.Image, MaxPages+1)
	for i := range pages {
		pages[i] = image.NewGray(image.Rect(0, 0, 1, 1))
	}
	if _, err := Inspect(context.Background(), tifffixture.Encode(pages, nil, binary.LittleEndian)); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func FuzzTIFFBoundedDecode(f *testing.F) {
	f.Add(tifffixture.TwoPages())
	f.Add([]byte("II*\x00"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		pages, err := Inspect(context.Background(), data)
		if err != nil {
			return
		}
		for _, p := range pages {
			if p.Width*p.Height > 65536 {
				return
			}
			_, _ = Decode(context.Background(), data, p)
		}
	})
}
