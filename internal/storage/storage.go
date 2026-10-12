package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileStorageService manages document files on the local filesystem.
// Files are stored as <storageDir>/<vehicleID>/<docID> (UUID, no user-controlled path component).
// All access must go through authenticated API endpoints — the directory is never exposed via HTTP.
type FileStorageService struct {
	storageDir string
}

// NewFileStorageService creates a new FileStorageService and ensures the root storage directory exists.
func NewFileStorageService(storageDir string) (*FileStorageService, error) {
	if err := os.MkdirAll(storageDir, 0o750); err != nil {
		return nil, fmt.Errorf("storage: cannot create storage directory %q: %w", storageDir, err)
	}
	return &FileStorageService{storageDir: storageDir}, nil
}

// part rejects an identifier that is not a single path element.
func part(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}

// resolve turns a relative storage path into a path inside the storage root. It refuses anything that is not a
// plain relative path (absolute, "..", the root itself) before the join, so a sibling directory that merely shares
// the root's name as a prefix cannot be reached.
func (s *FileStorageService) resolve(storagePath string) (string, error) {
	if !filepath.IsLocal(storagePath) || filepath.Clean(storagePath) == "." {
		return "", fmt.Errorf("storage: path traversal detected for %q", storagePath)
	}
	return filepath.Join(s.storageDir, filepath.Clean(storagePath)), nil
}

// Save writes document bytes to <storageDir>/<vehicleID>/<docID>.
// vehicleID and docID are UUID strings from the database; each must still be a single path element.
// Returns the relative storage path stored in the database (vehicleID/docID).
func (s *FileStorageService) Save(vehicleID, docID string, data []byte) (string, error) {
	if !part(vehicleID) || !part(docID) {
		return "", fmt.Errorf("storage: invalid vehicle or document identifier")
	}
	rel := vehicleID + "/" + docID
	filePath, err := s.resolve(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		return "", fmt.Errorf("storage: cannot create vehicle dir %q: %w", filepath.Dir(filePath), err)
	}
	if err := os.WriteFile(filePath, data, 0o640); err != nil {
		return "", fmt.Errorf("storage: cannot write file %q: %w", filePath, err)
	}
	return rel, nil
}

// Read reads document bytes from the given relative storage path (vehicleID/docID from the database).
func (s *FileStorageService) Read(storagePath string) ([]byte, error) {
	absPath, err := s.resolve(storagePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("storage: cannot read file %q: %w", absPath, err)
	}
	return data, nil
}

// Delete removes the document file. Returns nil if the file does not exist (idempotent).
func (s *FileStorageService) Delete(storagePath string) error {
	absPath, err := s.resolve(storagePath)
	if err != nil {
		return err
	}
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: cannot delete file %q: %w", absPath, err)
	}
	return nil
}
