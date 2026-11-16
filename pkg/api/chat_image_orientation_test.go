package api

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func orientedPNGFixture(t *testing.T, kind string) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := 0; i < 6; i++ {
		img.SetNRGBA(i%3, i/3, color.NRGBA{R: uint8(i + 1), A: 255})
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	exif := make([]byte, 26)
	copy(exif, "II")
	binary.LittleEndian.PutUint16(exif[2:], 42)
	binary.LittleEndian.PutUint32(exif[4:], 8)
	binary.LittleEndian.PutUint16(exif[8:], 1)
	binary.LittleEndian.PutUint16(exif[10:], 0x112)
	binary.LittleEndian.PutUint16(exif[12:], 3)
	binary.LittleEndian.PutUint32(exif[14:], 1)
	binary.LittleEndian.PutUint16(exif[18:], 6)
	chunk := make([]byte, len(exif)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(exif)))
	copy(chunk[4:], kind)
	copy(chunk[8:], exif)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	data := append(append(append([]byte{}, raw.Bytes()[:33]...), chunk...), raw.Bytes()[33:]...)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func TestChatImageOrientationProjectionPreservesOriginal(t *testing.T) {
	url := orientedPNGFixture(t, "eXIf")
	messages := []ChatMessage{{Role: "user", Content: SessionContent("Read this", []SessionImage{{Name: "scan.png", DataURL: url}})}}
	before, _ := json.Marshal(messages)
	prepared, err := PrepareChatImages(messages)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(messages)
	if !bytes.Equal(before, after) {
		t.Fatal("original request mutated")
	}
	parts := prepared[0].Content.([]ChatContentPart)
	projection := parts[1].ImageURL.URL
	if projection == url || parts[0].Text != "Read this" {
		t.Fatal("orientation not projected")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(projection, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 3 {
		t.Fatalf("bad projected size %v", img.Bounds())
	}
	var values []byte
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			values = append(values, byte('0'+r/257))
		}
	}
	if string(values) != "415263" {
		t.Fatalf("wrong pixels %s", values)
	}
	again, err := PrepareChatImages(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Content.([]ChatContentPart)[1].ImageURL.URL != projection {
		t.Fatal("projection applied twice")
	}
	if err := ValidateSessionImages([]SessionImage{{Name: "scan.png", DataURL: url}}); err != nil {
		t.Fatal(err)
	}
}

func TestChatRejectsAnimatedPNGOnBothPaths(t *testing.T) {
	url := orientedPNGFixture(t, "acTL")
	if err := ValidateSessionImages([]SessionImage{{Name: "animated.png", DataURL: url}}); err == nil {
		t.Fatal("saved Chat accepted animation")
	}
	messages := []ChatMessage{{Role: "user", Content: []ChatContentPart{{Type: "image_url", ImageURL: &ChatImageURL{URL: url}}}}}
	if _, err := ValidateChatMessages(messages); err == nil {
		t.Fatal("generic Chat accepted animation")
	}
	if _, err := PrepareChatImages(messages); err == nil {
		t.Fatal("runtime projection accepted animation")
	}
}

func TestOrientedImageOutputIsBounded(t *testing.T) {
	var out imageOutput
	if n, err := out.Write(make([]byte, maxOrientedImageBytes+1)); n != 0 || err == nil || out.Len() != 0 {
		t.Fatal("oversized encoding was retained")
	}
}
