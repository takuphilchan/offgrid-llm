package storage

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupRestoreWholeWorkspace(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(data, "rag"), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := OpenSQLite(filepath.Join(data, "rag", "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE records(id INTEGER PRIMARY KEY, text TEXT); INSERT INTO records VALUES(1, 'Nguva · 中文')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sessions/chat.json", "agent_tasks/task.json", "runs/events.jsonl", "artifacts/sha256", "users.json", "资料/文档.txt"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(data, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(data, name), []byte("private: "+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	backup := filepath.Join(root, "backup.zip")
	manifest, err := BackupWorkspace(context.Background(), data, backup, "0.4.3")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 7 {
		t.Fatalf("incomplete backup: %+v", manifest)
	}
	if _, err := VerifyBackup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored")
	if _, err := RestoreWorkspace(context.Background(), backup, target, "0.4.3"); err != nil {
		t.Fatal(err)
	}
	db, err = OpenSQLite(filepath.Join(target, "rag", "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var text string
	if err := db.QueryRow("SELECT text FROM records WHERE id=1").Scan(&text); err != nil || text != "Nguva · 中文" {
		t.Fatalf("database contents: %q %v", text, err)
	}
	for _, item := range manifest.Files {
		if item.Path == "rag/rag.db" {
			continue
		} // SQLite may checkpoint during verification.
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(item.Path)))
		if err != nil || string(got) != "private: "+item.Path {
			t.Fatalf("%s: %q %v", item.Path, got, err)
		}
	}
	if _, err := RestoreWorkspace(context.Background(), backup, target, "0.4.3"); err == nil {
		t.Fatal("overwrote existing workspace")
	}
	if _, err := BackupWorkspace(context.Background(), data, backup, "0.4.3"); err == nil {
		t.Fatal("overwrote backup")
	}
}

func TestBackupRequiresStoppedServiceAndOutsideDestination(t *testing.T) {
	root := t.TempDir()
	owner, err := AcquireOwnership(root)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := BackupWorkspace(context.Background(), root, filepath.Join(t.TempDir(), "backup.zip"), "0.4.3"); !errors.Is(err, ErrWorkspaceInUse) {
		t.Fatalf("live backup allowed: %v", err)
	}
	owner.Close()
	if _, err := BackupWorkspace(context.Background(), root, filepath.Join(root, "backup.zip"), "0.4.3"); err == nil {
		t.Fatal("backup inside workspace allowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BackupWorkspace(ctx, root, filepath.Join(t.TempDir(), "backup.zip"), "0.4.3"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func makeBackupFixture(t *testing.T, name string, data []byte, manifestChange func(*BackupManifest)) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "test.zip")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	w, err := archive.Create("state/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	manifest := BackupManifest{Format: 1, ApplicationVersion: "0.4.3", CreatedAt: time.Now(), Scope: "workspace-data-directory", Bytes: int64(len(data)), Files: []BackupFile{{Path: name, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}}}
	if manifestChange != nil {
		manifestChange(&manifest)
	}
	w, err = archive.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestBackupRejectsTraversalAndTampering(t *testing.T) {
	for _, name := range []string{"../outside", "/outside", `x\..\outside`, "x/C:/outside", "CON.txt", "x/../file", "x/file.", "x/file ", ".offgrid-owner.lock"} {
		t.Run(name, func(t *testing.T) {
			archive := makeBackupFixture(t, name, []byte("data"), nil)
			if _, err := VerifyBackup(context.Background(), archive); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	for _, change := range []func(*BackupManifest){
		func(m *BackupManifest) {
			m.Files[0].SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		},
		func(m *BackupManifest) { m.Files = append(m.Files, m.Files[0]) },
		func(m *BackupManifest) { m.Bytes++ },
		func(m *BackupManifest) { m.Format = 99 },
	} {
		archive := makeBackupFixture(t, "file", []byte("data"), change)
		if _, err := VerifyBackup(context.Background(), archive); err == nil {
			t.Fatal("damaged manifest accepted")
		}
	}
}

func TestRestoreRejectsCorruptSQLiteAndVersionMismatch(t *testing.T) {
	archive := makeBackupFixture(t, "rag/rag.db", []byte("not a database"), nil)
	root := t.TempDir()
	target := filepath.Join(root, "restored")
	if _, err := RestoreWorkspace(context.Background(), archive, target, "0.4.3"); err == nil {
		t.Fatal("corrupt database restored")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("failed restore activated target")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging not cleaned: %v %v", entries, err)
	}
	archive = makeBackupFixture(t, "file", []byte("data"), nil)
	if _, err := RestoreWorkspace(context.Background(), archive, target, "0.4.2"); err == nil {
		t.Fatal("mismatched application restored")
	}
}

func TestPublishDirectoryDoesNotReplaceExisting(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "stage"), filepath.Join(root, "target")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := publishDirectory(source, target); err == nil {
		t.Fatal("existing empty directory replaced")
	}
}

func TestBackupPathAliases(t *testing.T) {
	for _, pair := range [][2]string{{"Docs/a", "docs/b"}, {"café/a", "cafe\u0301/b"}, {"a", "a/b"}, {"a/b", "a"}, {"a/b", "a/b"}} {
		paths := make(backupPaths)
		if err := paths.add(pair[0]); err != nil {
			t.Fatal(err)
		}
		if err := paths.add(pair[1]); err == nil {
			t.Fatalf("alias accepted: %v", pair)
		}
	}
	paths := make(backupPaths)
	for _, name := range []string{"Docs/a", "Docs/b", "资料/文档.txt"} {
		if err := paths.add(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBackupWALCrashHelper(t *testing.T) {
	filename := os.Getenv("OFFGRID_BACKUP_WAL_TEST")
	if filename == "" {
		return
	}
	db, err := OpenSQLite(filename)
	if err != nil {
		os.Exit(20)
	}
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=0; CREATE TABLE committed(value TEXT); INSERT INTO committed VALUES('acknowledged before crash')"); err != nil {
		os.Exit(21)
	}
	// Leave the WAL in place, exactly as an ungraceful process exit can do.
	os.Exit(0)
}

func TestBackupRestoresCommittedWALAfterCrash(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(data, "workspace.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackupWALCrashHelper$")
	cmd.Env = append(os.Environ(), "OFFGRID_BACKUP_WAL_TEST="+filename)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("crash fixture: %v %s", err, out)
	}
	if info, err := os.Stat(filename + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("missing WAL fixture: %v", err)
	}
	archive := filepath.Join(root, "backup.zip")
	if _, err := BackupWorkspace(context.Background(), data, archive, "0.4.3"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored")
	if _, err := RestoreWorkspace(context.Background(), archive, target, "0.4.3"); err != nil {
		t.Fatal(err)
	}
	db, err := OpenSQLite(filepath.Join(target, "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("SELECT value FROM committed").Scan(&result); err != nil || result != "acknowledged before crash" {
		t.Fatalf("lost acknowledged WAL commit: %q %v", result, err)
	}
}

func TestSQLiteForeignKeyVerification(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	connection, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.ExecContext(context.Background(), "PRAGMA foreign_keys=OFF; CREATE TABLE parent(id INTEGER PRIMARY KEY); CREATE TABLE child(id INTEGER REFERENCES parent(id)); INSERT INTO child VALUES(42)"); err != nil {
		t.Fatal(err)
	}
	connection.Close()
	if err := CheckSQLite(context.Background(), db); err == nil {
		t.Fatal("orphaned row passed integrity verification")
	}
}
