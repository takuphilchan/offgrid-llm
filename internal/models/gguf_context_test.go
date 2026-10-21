package models

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadGGUFContextProfile(t *testing.T) {
	var data bytes.Buffer
	data.WriteString("GGUF")
	write := func(value any) {
		t.Helper()
		if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	write(uint32(3))
	write(uint64(0))
	entries := []struct {
		key   string
		kind  uint32
		value any
	}{
		{"general.architecture", 8, "qwen2"},
		{"qwen2.context_length", 4, uint32(131072)},
		{"qwen2.block_count", 4, uint32(28)},
		{"qwen2.embedding_length", 4, uint32(3584)},
		{"qwen2.attention.head_count", 4, uint32(28)},
		{"qwen2.attention.head_count_kv", 4, uint32(4)},
	}
	write(uint64(len(entries)))
	for _, entry := range entries {
		write(uint64(len(entry.key)))
		data.WriteString(entry.key)
		write(entry.kind)
		if entry.kind == 8 {
			value := entry.value.(string)
			write(uint64(len(value)))
			data.WriteString(value)
		} else {
			write(entry.value)
		}
	}
	profile, err := readGGUFContextProfile(bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if profile.Architecture != "qwen2" || profile.ContextLength != 131072 || profile.KVElementsPerToken != 28672 {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestReadGGUFContextProfileRejectsUnmeasuredHeader(t *testing.T) {
	for _, data := range [][]byte{[]byte("nope"), append([]byte("GGUF"), 9, 0, 0, 0)} {
		if _, err := readGGUFContextProfile(bytes.NewReader(data)); err == nil {
			t.Fatal("invalid metadata was accepted")
		}
	}
}
