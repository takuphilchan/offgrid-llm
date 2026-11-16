package sessions

import "testing"

func TestMixedOCRAttachmentPersistsAcrossRestartAndAccessBoundaries(t *testing.T) {
	dir := t.TempDir()
	scoped := NewSessionManager(dir).WithAccess(Access{UserID: "alice"})
	if _, err := scoped.Create("private", "model"); err != nil {
		t.Fatal(err)
	}
	a, err := scoped.CreateAttachmentWithOCRLanguage("private", "scan.png", "image/png", []byte("source bytes"), "chi_sim+fra")
	if err != nil {
		t.Fatal(err)
	}
	reopened := NewSessionManager(dir)
	got, _, err := reopened.WithAccess(Access{UserID: "alice"}).AttachmentOriginal("private", a.ID)
	if err != nil || got.OCRLanguage != "chi_sim+fra" || got.SHA256 != a.SHA256 {
		t.Fatal(got, err)
	}
	if _, _, err = reopened.WithAccess(Access{UserID: "bob"}).AttachmentOriginal("private", a.ID); err == nil {
		t.Fatal("mixed attachment leaked")
	}
	for _, selection := range []string{"eng+eng", "eng+sn", "eng+fra+spa+deu"} {
		if _, err = scoped.CreateAttachmentWithOCRLanguage("private", "bad.png", "image/png", []byte("source"), selection); err == nil {
			t.Fatal("accepted invalid selection", selection)
		}
	}
}
