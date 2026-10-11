package computer

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestControlMessageRejectsLossyOrAmbiguousTextBeforeDispatch(t *testing.T) {
	for _, raw := range []string{
		`{"kind":"replace_text","element":"editor","text":"\ud800"}`,
		`{"kind":"replace_text","element":"editor","text":"\udc00"}`,
		"{\"kind\":\"replace_text\",\"element\":\"editor\",\"text\":\"\xff\"}",
		`{"kind":"replace_text","element":"editor","text":"first","text":"second"}`,
		`{"kind":"replace_text","element":"editor","text":"first","\u0074ext":"second"}`,
	} {
		var op Operation
		if err := DecodeControlMessage([]byte(raw), &op); err == nil {
			t.Fatalf("ambiguous input accepted: %q", raw)
		}
		var frame bytes.Buffer
		if err := binary.Write(&frame, binary.BigEndian, uint32(len(raw))); err != nil {
			t.Fatal(err)
		}
		frame.WriteString(raw)
		if err := ReadControlFrame(&frame, &op); err == nil {
			t.Fatal("framed input bypassed Unicode validation")
		}
	}
}

func TestControlMessagePreservesExactValidText(t *testing.T) {
	for _, raw := range []string{
		`{"kind":"replace_text","element":"editor","text":"OffGrid — café 日本語 😀 e\u0301\r\n"}`,
		`{"kind":"replace_text","element":"editor","text":"\ud83d\ude00"}`,
		`{"kind":"replace_text","element":"editor","text":"�"}`,
		`{"kind":"replace_text","element":"editor","text":"\\uD800"}`,
	} {
		var op Operation
		if err := DecodeControlMessage([]byte(raw), &op); err != nil || op.Validate() != nil {
			t.Fatalf("valid Unicode input rejected: %q: %v", raw, err)
		}
	}
}
