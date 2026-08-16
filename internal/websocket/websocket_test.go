package websocket

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestReadFrameRequiresMask(t *testing.T) {
	conn := &Connection{}
	_, _, err := conn.readFrame(bufio.NewReader(bytes.NewReader([]byte{0x81, 0x01, 'x'})))
	if err == nil {
		t.Fatal("unmasked client frame was accepted")
	}
}

func TestConnectionMessageRateLimit(t *testing.T) {
	now := time.Now()
	conn := &Connection{windowStart: now}
	for i := 0; i < maxMessagesPerWindow; i++ {
		if !conn.allowMessage(now) {
			t.Fatalf("message %d was rejected before the burst limit", i+1)
		}
	}
	if conn.allowMessage(now) {
		t.Fatal("message above the burst limit was accepted")
	}
	if !conn.allowMessage(now.Add(messageRateWindow)) {
		t.Fatal("message window did not reset")
	}
}

func TestReadFrameRejectsOversizedPayload(t *testing.T) {
	frame := []byte{0x81, 0x80 | 127}
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(maxPayloadSize+1))
	frame = append(frame, length...)

	conn := &Connection{}
	_, _, err := conn.readFrame(bufio.NewReader(bytes.NewReader(frame)))
	if err == nil {
		t.Fatal("oversized websocket frame was accepted")
	}
}

func TestReadFrameUnmasksPayload(t *testing.T) {
	mask := []byte{1, 2, 3, 4}
	payload := []byte("hello")
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%len(mask)]
	}
	frame := append([]byte{0x81, 0x80 | byte(len(payload))}, mask...)
	frame = append(frame, masked...)

	conn := &Connection{}
	messageType, got, err := conn.readFrame(bufio.NewReader(bytes.NewReader(frame)))
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if messageType != TextMessage || string(got) != string(payload) {
		t.Fatalf("readFrame = (%v, %q), want (%v, %q)", messageType, got, TextMessage, payload)
	}
}
