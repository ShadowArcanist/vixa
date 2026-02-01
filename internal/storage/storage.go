package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Storage struct {
	basePath string
	mu       sync.RWMutex
}

func NewStorage(basePath string) (*Storage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &Storage{
		basePath: basePath,
	}, nil
}

func (s *Storage) StoreFile(domainFolder, category string, data []byte, contentType, ext string) (filename string, size int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate unique filename
	id := uuid.New().String()
	filename = id + ext

	// Create directory structure: /basePath/domainFolder/category/
	dirPath := filepath.Join(s.basePath, domainFolder, category)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	filePath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write file: %w", err)
	}

	return filename, len(data), nil
}

func (s *Storage) GetFile(domainFolder, category, filename string) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.basePath, domainFolder, category, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	// Detect content type
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return data, contentType, nil
}

func (s *Storage) DeleteFile(domainFolder, category, filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.basePath, domainFolder, category, filename)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found")
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (s *Storage) ListFiles(domainFolder, category string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dirPath := filepath.Join(s.basePath, domainFolder, category)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	// Sort files alphabetically
	sort.Strings(files)

	return files, nil
}

func DownloadFile(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Clean up content type (remove charset, etc.)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return data, contentType, nil
}

func GenerateETag(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf(`"%s"`, hex.EncodeToString(hash[:8]))
}
