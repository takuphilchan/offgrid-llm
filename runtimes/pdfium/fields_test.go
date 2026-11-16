package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// Synthetic licensed fixture: no personal documents or form actions are run.
func acroFormPDF(extra ...string) []byte {
	objects := []string{
		`<< /Type /Catalog /Pages 2 0 R /AcroForm << /Fields [5 0 R 6 0 R 7 0 R 8 0 R] /NeedAppearances true >> >>`,
		`<< /Type /Pages /Kids [3 0 R] /Count 1 >>`,
		`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Annots [5 0 R 6 0 R 7 0 R 8 0 R] /Contents 4 0 R /Resources << >> >>`,
		"<< /Length 0 >>\nstream\n\nendstream",
		`<< /Type /Annot /Subtype /Widget /FT /Tx /T (Amount) /V (0017.25) /Rect [10 20 80 50] /P 3 0 R >>`,
		`<< /Type /Annot /Subtype /Widget /FT /Btn /T (Approved) /V /Yes /AS /Yes /Rect [10 60 30 80] /P 3 0 R /AP << /N << /Yes 9 0 R /Off 9 0 R >> >> >>`,
		`<< /Type /Annot /Subtype /Widget /FT /Btn /T (Rejected) /V /Off /AS /Off /Rect [40 60 60 80] /P 3 0 R /AP << /N << /Yes 9 0 R /Off 9 0 R >> >> >>`,
		`<< /Type /Annot /Subtype /Widget /FT /Tx /T (Hidden) /V (Draft) /F 2 /Rect [10 90 80 110] /P 3 0 R >>`,
		"<< /Type /XObject /Subtype /Form /BBox [0 0 20 20] /Length 0 >>\nstream\n\nendstream",
	}
	for _, obj := range extra {
		index := len(objects) + 1
		for _, i := range []int{0, 2} {
			objects[i] = strings.Replace(objects[i], "5 0 R 6 0 R 7 0 R 8 0 R", fmt.Sprintf("5 0 R 6 0 R 7 0 R 8 0 R %d 0 R", index), 1)
		}
		objects = append(objects, obj)
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	start := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, o := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), start)
	return b.Bytes()
}

func TestNativeFormMultipleSelectionsAndSignature(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	data := acroFormPDF(
		`<< /Type /Annot /Subtype /Widget /FT /Ch /Ff 2097152 /T (Regions) /TU (Service areas) /Opt [(North) (South) (West)] /V [(North) (West)] /I [0 2] /Rect [10 120 80 180] /P 3 0 R >>`,
		`<< /Type /Annot /Subtype /Widget /FT /Sig /T (Approval signature) /V << /Type /Sig /Contents (not-authenticated) >> /Rect [90 20 180 80] /P 3 0 R >>`)
	d, code := openPDF(context.Background(), data)
	if code != "" {
		t.Fatal(code)
	}
	defer d.close()
	form, close, code := d.formEnvironment()
	if code != "" {
		t.Fatal(code)
	}
	defer close()
	page, err := d.engine.call("FPDF_LoadPage", d.handle, 0)
	if err != nil || page == 0 {
		t.Fatal(err)
	}
	defer d.engine.call("FPDF_ClosePage", page)
	d.engine.call("FORM_OnAfterLoadPage", page, form)
	defer d.engine.call("FORM_OnBeforeClosePage", page, form)
	fields, code := readFormFields(d.engine, page, form)
	if code != "" || len(fields) != 6 {
		t.Fatalf("%s %+v", code, fields)
	}
	var choice, signature formField
	for _, f := range fields {
		if f.Kind == 5 {
			choice = f
		}
		if f.Kind == 7 {
			signature = f
		}
	}
	if choice.Label != "Service areas" || len(choice.Options) != 3 || !choice.Options[0].Selected || choice.Options[1].Selected || !choice.Options[2].Selected || choice.Options[2].Label != "West" {
		t.Fatalf("lost selections: %+v", choice)
	}
	if signature.Kind != 7 || signature.Value != "" {
		t.Fatal("signature interpreted as authentication", signature)
	}
}

func TestNativeFormValuesAndCheckboxes(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	d, code := openPDF(context.Background(), acroFormPDF())
	if code != "" {
		t.Fatal(code)
	}
	defer d.close()
	form, close, code := d.formEnvironment()
	if code != "" {
		t.Fatal(code)
	}
	defer close()
	page, err := d.engine.call("FPDF_LoadPage", d.handle, 0)
	if err != nil || page == 0 {
		t.Fatal(err)
	}
	defer d.engine.call("FPDF_ClosePage", page)
	d.engine.call("FORM_OnAfterLoadPage", page, form)
	defer d.engine.call("FORM_OnBeforeClosePage", page, form)
	fields, code := readFormFields(d.engine, page, form)
	if code != "" || len(fields) != 4 {
		t.Fatalf("%s %+v", code, fields)
	}
	if fields[0].Name != "Amount" || fields[0].Value != "0017.25" || fields[0].Kind != 6 || fields[0].Bounds == nil || *fields[0].Bounds != [4]float64{10, 20, 80, 50} {
		t.Fatalf("exact native value/rectangle: %+v", fields[0])
	}
	if fields[1].Checked == nil || !*fields[1].Checked || fields[2].Checked == nil || *fields[2].Checked || !fields[3].Hidden {
		t.Fatalf("native states: checked=%v/%v hidden=%v", *fields[1].Checked, *fields[2].Checked, fields[3].Hidden)
	}
}
