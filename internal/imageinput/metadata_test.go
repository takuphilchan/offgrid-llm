package imageinput

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func exifFixture(order binary.ByteOrder, orientation uint16) []byte {
	b := make([]byte, 26)
	copy(b, "II")
	if order == binary.BigEndian {
		copy(b, "MM")
	}
	order.PutUint16(b[2:], 42)
	order.PutUint32(b[4:], 8)
	order.PutUint16(b[8:], 1)
	order.PutUint16(b[10:], 0x112)
	order.PutUint16(b[12:], 3)
	order.PutUint32(b[14:], 1)
	order.PutUint16(b[18:], orientation)
	return b
}

func pngChunk(kind string, data []byte) []byte {
	chunk := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(data)))
	copy(chunk[4:], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	return chunk
}

func TestMetadataOrientationAndAmbiguity(t *testing.T) {
	var jpg, p bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&p, img); err != nil {
		t.Fatal(err)
	}
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for orientation := 1; orientation <= 8; orientation++ {
			exif := exifFixture(order, uint16(orientation))
			segment := append([]byte{255, 225, 0, byte(len(exif) + 8)}, []byte("Exif\x00\x00")...)
			segment = append(segment, exif...)
			jpegData := append(append(append([]byte{}, jpg.Bytes()[:2]...), segment...), jpg.Bytes()[2:]...)
			pngData := append(append(append([]byte{}, p.Bytes()[:33]...), pngChunk("eXIf", exif)...), p.Bytes()[33:]...)
			end := p.Len() - 12
			late := append(append(append([]byte{}, p.Bytes()[:end]...), pngChunk("eXIf", exif)...), p.Bytes()[end:]...)
			if got, err := Orientation(late, "png"); err != nil || got != orientation {
				t.Fatalf("late PNG orientation %d: got %d, %v", orientation, got, err)
			}
			for format, data := range map[string][]byte{"jpeg": jpegData, "png": pngData} {
				got, err := Orientation(data, format)
				if err != nil || got != orientation {
					t.Fatalf("%s %d: got %d, %v", format, orientation, got, err)
				}
			}
		}
	}
	for _, tag := range []uint16{0, 9, 65535} {
		if _, err := exifOrientation(exifFixture(binary.LittleEndian, tag)); err == nil {
			t.Fatalf("accepted orientation %d", tag)
		}
	}
	for _, mutation := range []func([]byte){
		func(b []byte) { binary.LittleEndian.PutUint32(b[4:], 0xffffffff) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[8:], 65535) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[12:], 4) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[14:], 2) },
	} {
		b := exifFixture(binary.LittleEndian, 6)
		mutation(b)
		if _, err := exifOrientation(b); err == nil {
			t.Fatal("accepted malformed orientation directory")
		}
	}
	for _, kind := range []string{"acTL", "fcTL", "fdAT"} {
		b := append(append(append([]byte{}, p.Bytes()[:33]...), pngChunk(kind, make([]byte, 8))...), p.Bytes()[33:]...)
		if _, err := Orientation(b, "png"); !errors.Is(err, ErrAnimation) {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	duplicate := append(pngChunk("eXIf", exifFixture(binary.LittleEndian, 6)), pngChunk("eXIf", exifFixture(binary.LittleEndian, 6))...)
	b := append(append(append([]byte{}, p.Bytes()[:33]...), duplicate...), p.Bytes()[33:]...)
	if _, err := Orientation(b, "png"); !errors.Is(err, ErrMetadata) {
		t.Fatal("duplicate Exif accepted")
	}
	mpo := append(append(append([]byte{}, jpg.Bytes()[:2]...), []byte{255, 226, 0, 6, 'M', 'P', 'F', 0}...), jpg.Bytes()[2:]...)
	if _, err := Orientation(mpo, "jpeg"); !errors.Is(err, ErrAnimation) {
		t.Fatal("multi-picture JPEG accepted")
	}
}

func TestOrientAllEightTransformsPreservePixels(t *testing.T) {
	// The coordinate-labelled nonsquare fixture detects rotation/mirroring swaps.
	source := image.NewNRGBA64(image.Rect(5, 7, 8, 9))
	for i := 0; i < 6; i++ {
		source.SetNRGBA64(5+i%3, 7+i/3, color.NRGBA64{R: uint16(i + 1), G: 50001, B: 65535, A: 65535})
	}
	expected := []string{"123456", "321654", "654321", "456123", "142536", "415263", "635241", "362514"}
	for orientation := 1; orientation <= 8; orientation++ {
		got, err := Orient(source, orientation)
		if err != nil {
			t.Fatal(err)
		}
		var labels []byte
		for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
			for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
				c := color.NRGBA64Model.Convert(got.At(x, y)).(color.NRGBA64)
				if c.G != 50001 || c.B != 65535 || c.A != 65535 {
					t.Fatal("16-bit channels changed")
				}
				labels = append(labels, byte('0'+c.R))
			}
		}
		if string(labels) != expected[orientation-1] {
			t.Fatalf("orientation %d: %s", orientation, labels)
		}
		if orientation >= 5 && (got.Bounds().Dx() != 2 || got.Bounds().Dy() != 3) {
			t.Fatal("dimensions not swapped")
		}
	}
}

func FuzzImageMetadata(f *testing.F) {
	f.Add(exifFixture(binary.LittleEndian, 6))
	f.Add([]byte{255, 216, 255, 217})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 8<<20 {
			t.Skip()
		}
		for _, format := range []string{"jpeg", "png"} {
			orientation, err := Orientation(b, format)
			if err == nil && (orientation < 1 || orientation > 8) {
				t.Fatal("invalid orientation returned")
			}
		}
		_, _ = exifOrientation(b)
	})
}
