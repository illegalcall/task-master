package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage(t *testing.T) {
	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create storage instance
	storage, err := NewLocalStorage(tempDir)
	require.NoError(t, err)

	t.Run("StoreFromURL", func(t *testing.T) {
		// Create test server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("%PDF-1.4\nTest PDF content"))
		}))
		defer ts.Close()

		// Test storing from URL
		ctx := context.Background()
		path, err := storage.StoreFromURL(ctx, ts.URL)
		require.NoError(t, err)
		assert.True(t, filepath.HasPrefix(path, tempDir))
		
		// Verify file content
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(content), "%PDF-1.4")
		
		// Clean up
		require.NoError(t, storage.Delete(ctx, path))
	})

	t.Run("StoreFromBytes", func(t *testing.T) {
		ctx := context.Background()
		testData := []byte("%PDF-1.4\nTest PDF content")

		// Test storing bytes
		path, err := storage.StoreFromBytes(ctx, testData)
		require.NoError(t, err)
		assert.True(t, filepath.HasPrefix(path, tempDir))

		// Verify file content
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, testData, content)

		// Clean up
		require.NoError(t, storage.Delete(ctx, path))
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		
		// Create test file
		path, err := storage.StoreFromBytes(ctx, []byte("test"))
		require.NoError(t, err)

		// Test deletion
		require.NoError(t, storage.Delete(ctx, path))
		_, err = os.Stat(path)
		assert.True(t, os.IsNotExist(err))

		// Test deleting non-existent file
		err = storage.Delete(ctx, "nonexistent")
		assert.Error(t, err)

		// Test deleting file outside temp directory
		err = storage.Delete(ctx, "/tmp/outside")
		assert.Error(t, err)
	})
}

func TestNewLocalStorage(t *testing.T) {
	t.Run("Valid directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "storage-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		storage, err := NewLocalStorage(tempDir)
		assert.NoError(t, err)
		assert.NotNil(t, storage)
	})

	t.Run("Invalid directory", func(t *testing.T) {
		_, err := NewLocalStorage("/nonexistent/directory")
		if err == nil {
			// Some systems might allow creating directories in /nonexistent
			// In this case, clean up
			os.RemoveAll("/nonexistent/directory")
		}
	})
} 