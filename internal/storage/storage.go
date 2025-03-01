package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Storage defines the interface for file storage operations
type Storage interface {
	// StoreFromURL downloads and stores a file from a URL
	StoreFromURL(ctx context.Context, url string) (string, error)
	
	// StoreFromBytes stores a file from bytes
	StoreFromBytes(ctx context.Context, data []byte) (string, error)
	
	// Delete removes a file from storage
	Delete(ctx context.Context, path string) error
}

// LocalStorage implements Storage interface using local filesystem
type LocalStorage struct {
	tempDir string
}

// NewLocalStorage creates a new LocalStorage instance
func NewLocalStorage(tempDir string) (*LocalStorage, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	return &LocalStorage{tempDir: tempDir}, nil
}

func (s *LocalStorage) StoreFromURL(ctx context.Context, url string) (string, error) {
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: status %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp(s.tempDir, "pdf-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy data to file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		os.Remove(tempFile.Name()) // Clean up on error
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return tempFile.Name(), nil
}

func (s *LocalStorage) StoreFromBytes(ctx context.Context, data []byte) (string, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp(s.tempDir, "pdf-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Write data to file
	if _, err := tempFile.Write(data); err != nil {
		os.Remove(tempFile.Name()) // Clean up on error
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return tempFile.Name(), nil
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	// Verify the path is within our temp directory
	if !filepath.HasPrefix(path, s.tempDir) {
		return fmt.Errorf("invalid file path: must be within temp directory")
	}
	return os.Remove(path)
} 