package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teslacost/teslacost/internal/storage"
)

func TestFileStorageService(t *testing.T) {
	dir := t.TempDir()
	svc, err := storage.NewFileStorageService(dir)
	if err != nil {
		t.Fatalf("NewFileStorageService: %v", err)
	}

	vehicleID := "vehicle-uuid-123"
	docID := "doc-uuid-456"
	data := []byte("PDF content here")

	// Save
	path, err := svc.Save(vehicleID, docID, data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	expected := vehicleID + "/" + docID
	if path != expected {
		t.Fatalf("expected path %q, got %q", expected, path)
	}
	if _, err := os.Stat(filepath.Join(dir, vehicleID, docID)); err != nil {
		t.Fatalf("file not on disk: %v", err)
	}

	// Read
	got, err := svc.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("data mismatch: got %q", got)
	}

	// Path traversal rejected
	if _, err := svc.Read("../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}

	// Delete
	if err := svc.Delete(path); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, vehicleID, docID)); !os.IsNotExist(statErr) {
		t.Fatal("file should be deleted")
	}

	// Delete missing file is idempotent
	if err := svc.Delete(path); err != nil {
		t.Fatalf("Delete missing file should be idempotent: %v", err)
	}
}
func TestFileStorageServiceRefusesPathsOutsideTheRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "documents")
	sibling := filepath.Join(parent, "documents-other") // shares the root's name as a prefix
	for _, d := range []string{root, sibling} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret"), []byte("secret"), 0o640); err != nil {
		t.Fatal(err)
	}
	svc, err := storage.NewFileStorageService(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"../documents-other/secret", "a/../../documents-other/secret", filepath.Join(sibling, "secret"), "", ".", "/etc/passwd"} {
		if _, err := svc.Read(p); err == nil {
			t.Errorf("Read(%q) must be refused", p)
		}
		if err := svc.Delete(p); err == nil {
			t.Errorf("Delete(%q) must be refused", p)
		}
	}
	if _, err := os.Stat(filepath.Join(sibling, "secret")); err != nil {
		t.Fatalf("the sibling file must be untouched: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("the root must still exist: %v", err)
	}

	for _, id := range [][2]string{{"..", "doc"}, {"veh", ".."}, {"a/b", "doc"}, {"veh", "a/b"}, {"", "doc"}, {"veh", ""}, {"veh", "a\\b"}} {
		if _, err := svc.Save(id[0], id[1], []byte("x")); err == nil {
			t.Errorf("Save(%q, %q) must be refused", id[0], id[1])
		}
	}
}
