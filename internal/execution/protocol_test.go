package execution

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestStrictFrames(t *testing.T) {
	var reused bytes.Buffer
	WriteFrame(&reused, Response{Version: 1, ID: "old", Error: "denied"})
	WriteFrame(&reused, Response{Version: 1, ID: "new", Receipt: &Receipt{State: "complete"}})
	var destination Response
	if ReadFrame(&reused, &destination) != nil || ReadFrame(&reused, &destination) != nil || destination.Error != "" || destination.ID != "new" {
		t.Fatal("frame retained old fields", destination)
	}
	for _, body := range []string{`{"version":1,"version":2}`, `{"unknown":true}`, `{"id":"\ud800"}`, `{} {}`} {
		var b bytes.Buffer
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(body)))
		b.Write(header[:])
		b.WriteString(body)
		if err := ReadFrame(&b, new(Request)); err == nil {
			t.Fatal("accepted", body)
		}
	}
	for _, size := range []uint32{0, MaxFrameBytes + 1} {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], size)
		if ReadFrame(bytes.NewReader(b[:]), new(Request)) == nil {
			t.Fatal(size)
		}
	}
	var b bytes.Buffer
	if err := WriteFrame(&b, Response{Version: 1, ID: "req"}); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	if err := ReadFrame(bytes.NewReader(data[:len(data)-1]), new(Response)); err != io.ErrUnexpectedEOF {
		t.Fatal(err)
	}
	var response Response
	if err := ReadFrame(bytes.NewReader(data), &response); err != nil || response.ID != "req" {
		t.Fatal(response, err)
	}
}
