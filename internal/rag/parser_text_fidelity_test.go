package rag

import (
	"strings"
	"testing"
)

func TestParserJSONPreservesLargeIdentifiersAndDecimals(t *testing.T) {
	for _, value := range []string{"9007199254740993", "123456789012345678901234567890", "1.234567890123456789", "1e999"} {
		result, err := NewDocumentParser().Parse([]byte(`{"value":`+value+`}`), "record.json", ".json")
		if err != nil || !strings.Contains(result.Content, value) {
			t.Fatalf("JSON value changed: %s => %+v %v", value, result, err)
		}
	}
	duplicate, err := NewDocumentParser().Parse([]byte(`{"id":1,"id":2,"last":3}`), "duplicates.json", ".json")
	if err != nil || strings.Count(duplicate.Content, `"id"`) != 2 || strings.Index(duplicate.Content, `"last"`) < strings.Index(duplicate.Content, `"id"`) {
		t.Fatal("JSON members were reordered or discarded")
	}
	for _, value := range []string{`{} {}`, `{} invalid`} {
		if _, err := NewDocumentParser().Parse([]byte(value), "record.json", ".json"); err == nil {
			t.Fatal("trailing JSON accepted")
		}
	}
}

func TestParserStructuredTextRejectsInvalidUTF8(t *testing.T) {
	for _, ext := range []string{".html", ".xml", ".json", ".csv", ".go", ".py"} {
		if result, err := NewDocumentParser().Parse([]byte{'a', 0xff, 'b'}, "bad"+ext, ext); err == nil || result != nil {
			t.Fatalf("invalid text accepted as %s: %+v %v", ext, result, err)
		}
	}
}

func TestParserXMLDoesNotInventTextFromMalformedMarkup(t *testing.T) {
	p := NewDocumentParser()
	for _, source := range []string{"<doc><value>lost</doc>", "<one/> <two/>", "plain text", `<!DOCTYPE doc [<!ENTITY private SYSTEM "file:///private">]><doc>&private;</doc>`} {
		if _, err := p.Parse([]byte(source), "bad.xml", ".xml"); err == nil {
			t.Fatalf("invalid/unsupported XML accepted: %q", source)
		}
	}
	result, err := p.Parse([]byte(`<doc><value>café 日本語 &amp; &lt;tag&gt;</value><!-- private comment --><raw><![CDATA[<actual>]]></raw></doc>`), "source.xml", ".xml")
	if err != nil || !strings.Contains(result.Content, "café 日本語 & <tag>") || !strings.Contains(result.Content, "<actual>") || strings.Contains(result.Content, "private comment") {
		t.Fatalf("XML source fidelity: %+v %v", result, err)
	}
}
