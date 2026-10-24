package integrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentTransactionPreservesPrivateValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	initial := "# user note\nKEEP='user value'\nexport OFFGRID_API_KEY='old'\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	s := ConnectionStore{Directory: filepath.Join(dir, "state")}
	tx, err := s.Begin("hermes", []string{path})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	for i := 0; i < 2; i++ {
		if err = tx.SetEnvironment(path, map[string]string{"OFFGRID_API_KEY": "new'key"}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "# user note\nKEEP='user value'") || strings.Count(string(got), "OFFGRID_API_KEY=") != 1 || !strings.Contains(string(got), "new\\'key") {
		t.Fatalf("unexpected env: %q", got)
	}
	before, _ := os.ReadFile(filepath.Join(tx.Directory, "snapshot-0-0"))
	if string(before) != initial {
		t.Fatal("original recovery evidence lost")
	}
	if err = tx.SetEnvironment(path, map[string]string{"OFFGRID_API_KEY": "${SECRET}"}); err == nil {
		t.Fatal("dotenv expansion accepted")
	}
	if err = tx.SetEnvironment(filepath.Join(dir, "other"), map[string]string{"KEY": "value"}); err == nil {
		t.Fatal("unsnapshotted target accepted")
	}
}

func TestEnvironmentPreservesMultilineRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	keep := "MULTILINE='first\nOFFGRID_API_KEY=inside unrelated value\nlast'\n"
	initial := keep + "OFFGRID_API_KEY='old\nmultiline'\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	store := ConnectionStore{Directory: filepath.Join(dir, "state")}
	tx, err := store.Begin("hermes", []string{path})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	if err = tx.SetEnvironment(path, map[string]string{"OFFGRID_API_KEY": "new"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != keep+"\nOFFGRID_API_KEY='new'\n" {
		t.Fatalf("multiline record changed: %q", data)
	}
}
