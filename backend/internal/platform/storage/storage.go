// Package storage abstracts where export files live. The PRD calls for an
// S3-compatible bucket (R2 or MinIO); a local-disk driver is included so the
// app runs with zero external infrastructure during development.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store puts and gets export artefacts by key.
type Store interface {
	// Put writes the reader's content under key.
	Put(ctx context.Context, key string, r io.Reader, contentType string) error
	// Get opens the object for reading. Caller closes it.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes the object.
	Delete(ctx context.Context, key string) error
	// URL returns a link the browser can use. Local disk returns an API path.
	URL(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Driver names the backend for logging.
	Driver() string
}

// LocalStore keeps artefacts on the local filesystem.
type LocalStore struct {
	dir       string
	publicURL string
}

// NewLocal creates a disk-backed store rooted at dir.
func NewLocal(dir, publicURL string) (*LocalStore, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("storage: create dir: %w", err)
	}
	return &LocalStore{dir: abs, publicURL: strings.TrimRight(publicURL, "/")}, nil
}

// Put writes content under key, creating parent directories.
func (s *LocalStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("storage: mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("storage: create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage: write file: %w", err)
	}
	return nil
}

// Get opens the stored object.
func (s *LocalStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("storage: object not found: %s", key)
		}
		return nil, fmt.Errorf("storage: open: %w", err)
	}
	return f, nil
}

// Delete removes the stored object, ignoring a missing file.
func (s *LocalStore) Delete(ctx context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}

// URL returns the API download path for the object.
func (s *LocalStore) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("%s/api/v1/files/%s", s.publicURL, key), nil
}

// Driver names this store.
func (s *LocalStore) Driver() string { return "local" }

// path resolves key inside the store root, rejecting traversal attempts.
func (s *LocalStore) path(key string) (string, error) {
	clean := filepath.Clean("/" + key)
	full := filepath.Join(s.dir, clean)
	if !strings.HasPrefix(full, s.dir) {
		return "", fmt.Errorf("storage: key escapes root: %s", key)
	}
	return full, nil
}
