package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStructureUnitMetadataIsBoundedWithoutMutatingRevision(t *testing.T) {
	e := sampleExtraction()
	e.Units[0].MergedRanges = make([]string, 10000)
	for i := range e.Units[0].MergedRanges {
		e.Units[0].MergedRanges[i] = "A1:XFD1048576"
	}
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	page, err := ProjectStructure(e, "doc", "", 0, 100, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Units[0].MetadataOmitted || len(page.Units[0].MergedRanges) != 0 || len(e.Units[0].MergedRanges) != 10000 || e.Verify() != nil {
		t.Fatal("bounded projection mutated original or hid omission")
	}
	data, _ := json.Marshal(page)
	if len(data) > 8192 || !strings.Contains(string(data), `"metadata_omitted":true`) {
		t.Fatal("unbounded projection")
	}
}

func TestStructureRuntimeProvenanceIsRetainedBoundedAndCopied(t *testing.T) {
	e := sampleExtraction()
	e.Parsers = []RuntimeIdentity{{Name: "parser", Revision: "v1"}}
	e.LanguagePacks = []RuntimeIdentity{{Name: "tesseract-chi_sim", Revision: "pinned", SHA256: strings.Repeat("a", 64)}, {Name: "tesseract-fra", Revision: "pinned", SHA256: strings.Repeat("b", 64)}}
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	page, err := ProjectStructure(e, "doc", "", 0, 100, 0, false)
	if err != nil || page.ProvenanceOmitted || len(page.LanguagePacks) != 2 || len(page.Parsers) != 1 {
		t.Fatal(page, err)
	}
	page.LanguagePacks[0].Name = "mutated"
	if e.LanguagePacks[0].Name != "tesseract-chi_sim" || e.Verify() != nil {
		t.Fatal("projection mutated retained graph")
	}
	e.Parsers = make([]RuntimeIdentity, 10000)
	e.LanguagePacks[0].Revision = strings.Repeat("x", 1024)
	page, err = ProjectStructure(e, "doc", "", 0, 100, 0, false)
	if err != nil || !page.ProvenanceOmitted || len(page.Parsers) != 32 || len(page.LanguagePacks) != 1 {
		t.Fatal("unbounded or hidden omission", page, err)
	}
}
