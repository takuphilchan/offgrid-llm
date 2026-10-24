package models

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func templateFixture(t *testing.T, entries [][2]string) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteString("GGUF")
	write := func(v any) {
		t.Helper()
		if err := binary.Write(&out, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	write(uint32(3))
	write(uint64(0))
	write(uint64(len(entries)))
	for _, entry := range entries {
		write(uint64(len(entry[0])))
		out.WriteString(entry[0])
		write(uint32(8))
		write(uint64(len(entry[1])))
		out.WriteString(entry[1])
	}
	return out.Bytes()
}

func TestGGUFTemplateIdentity(t *testing.T) {
	entries := [][2]string{{"general.architecture", "llama"}, {"ignored", "other metadata"}, {"tokenizer.chat_template", "known template"}}
	data := templateFixture(t, entries)
	got, err := readGGUFTemplateIdentity(bytes.NewReader(data))
	if err != nil || got.Architecture != "llama" || got.Template != "known template" {
		t.Fatalf("%+v, %v", got, err)
	}
	for _, broken := range [][]byte{
		data[:len(data)-1], []byte("not GGUF"),
		templateFixture(t, append(entries, [2]string{"tokenizer.chat_template", "conflict"})),
		templateFixture(t, [][2]string{{"tokenizer.chat_template", strings.Repeat("x", 128<<10+1)}}),
	} {
		if _, err := readGGUFTemplateIdentity(bytes.NewReader(broken)); err == nil {
			t.Fatal("accepted invalid or ambiguous identity")
		}
	}
	missing, err := readGGUFTemplateIdentity(bytes.NewReader(templateFixture(t, entries[:1])))
	if err != nil || missing.Template != "" {
		t.Fatalf("unknown template should be empty: %+v %v", missing, err)
	}
}
