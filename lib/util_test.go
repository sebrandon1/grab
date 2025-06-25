package lib

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetLastModified_Success(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_setlastmod_success")

	// Create a test file
	testFile := "test_lastmod.txt"
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	_ = file.Close()

	// Create HTTP response with Last-Modified header
	lastModTime := time.Date(2023, 10, 15, 12, 30, 45, 0, time.UTC)
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("Last-Modified", lastModTime.Format(http.TimeFormat))

	// Test setLastModified
	err = setLastModified(resp, testFile)
	if err != nil {
		t.Errorf("setLastModified() returned error: %v", err)
	}

	// Verify the file timestamp was updated
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	actualTime := info.ModTime().UTC()
	expectedTime := lastModTime.UTC()

	// Allow for some time precision differences (within 1 second)
	if actualTime.Sub(expectedTime).Abs() > time.Second {
		t.Errorf("Expected mod time %v, got %v", expectedTime, actualTime)
	}
}

func TestSetLastModified_NoHeader(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_setlastmod_noheader")

	// Create a test file
	testFile := "test_noheader.txt"
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	_ = file.Close()

	// Get original mod time
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}
	originalTime := info.ModTime()

	// Create HTTP response without Last-Modified header
	resp := &http.Response{
		Header: make(http.Header),
	}

	// Test setLastModified (should be no-op)
	err = setLastModified(resp, testFile)
	if err != nil {
		t.Errorf("setLastModified() should not return error when no header present: %v", err)
	}

	// Verify the file timestamp was not changed
	info, err = os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	if !info.ModTime().Equal(originalTime) {
		t.Error("File mod time should not change when no Last-Modified header present")
	}
}

func TestSetLastModified_InvalidHeader(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_setlastmod_invalid")

	// Create a test file
	testFile := "test_invalid.txt"
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	_ = file.Close()

	// Create HTTP response with invalid Last-Modified header
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("Last-Modified", "invalid-date-format")

	// Test setLastModified (should handle gracefully)
	err = setLastModified(resp, testFile)
	if err != nil {
		t.Errorf("setLastModified() should handle invalid date gracefully: %v", err)
	}
}

func TestSetLastModified_NonExistentFile(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_setlastmod_nonexistent")

	// Create HTTP response with Last-Modified header
	lastModTime := time.Date(2023, 10, 15, 12, 30, 45, 0, time.UTC)
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("Last-Modified", lastModTime.Format(http.TimeFormat))

	// Test setLastModified on non-existent file
	err := setLastModified(resp, "nonexistent.txt")
	if err == nil {
		t.Error("setLastModified() should return error for non-existent file")
	}
}

func TestMkdirp_Success(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_mkdirp_success")

	// Test creating nested directories
	testPath := filepath.Join("level1", "level2", "level3", "test.txt")

	err := mkdirp(testPath)
	if err != nil {
		t.Errorf("mkdirp() returned error: %v", err)
	}

	// Verify directories were created
	expectedDir := filepath.Dir(testPath)
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Errorf("Expected directory %s was not created: %v", expectedDir, err)
	}

	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", expectedDir)
	}
}

func TestMkdirp_ExistingDirectory(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_mkdirp_existing")

	// Create a directory first
	existingDir := "existing"
	err := os.Mkdir(existingDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create existing directory: %v", err)
	}

	testPath := filepath.Join(existingDir, "test.txt")

	err = mkdirp(testPath)
	if err != nil {
		t.Errorf("mkdirp() should handle existing directory: %v", err)
	}
}

func TestMkdirp_FileInPath(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "util_mkdirp_fileinpath")

	// Create a file that will conflict with directory creation
	conflictFile := "conflict.txt"
	file, err := os.Create(conflictFile)
	if err != nil {
		t.Fatalf("Failed to create conflict file: %v", err)
	}
	_ = file.Close()

	// Try to create a path that would require the file to be a directory
	testPath := filepath.Join(conflictFile, "subdir", "test.txt")

	err = mkdirp(testPath)
	if err == nil {
		t.Error("mkdirp() should return error when file exists in path")
	}
}

func TestGuessFilename_FromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple filename",
			url:      "http://example.com/file.txt",
			expected: "file.txt",
		},
		{
			name:     "filename with path",
			url:      "http://example.com/path/to/document.pdf",
			expected: "document.pdf",
		},
		{
			name:     "filename with query params",
			url:      "http://example.com/download.zip?version=1.0",
			expected: "download.zip",
		},
		{
			name:     "complex path",
			url:      "http://example.com/downloads/software/v2.1/installer.exe",
			expected: "installer.exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			resp := &http.Response{
				Request: &http.Request{URL: parsedURL},
				Header:  make(http.Header),
			}

			filename, err := guessFilename(resp)
			if err != nil {
				t.Errorf("guessFilename() returned error: %v", err)
			}

			if filename != tt.expected {
				t.Errorf("Expected filename %q, got %q", tt.expected, filename)
			}
		})
	}
}

func TestGuessFilename_FromContentDisposition(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		disposition string
		expected    string
	}{
		{
			name:        "simple attachment",
			url:         "http://example.com/download",
			disposition: `attachment; filename="document.pdf"`,
			expected:    "document.pdf",
		},
		{
			name:        "inline with filename",
			url:         "http://example.com/view",
			disposition: `inline; filename="image.jpg"`,
			expected:    "image.jpg",
		},
		{
			name:        "filename with spaces",
			url:         "http://example.com/get",
			disposition: `attachment; filename="my document.docx"`,
			expected:    "my document.docx",
		},
		{
			name:        "quoted filename",
			url:         "http://example.com/file",
			disposition: `attachment; filename="report-2023.xlsx"`,
			expected:    "report-2023.xlsx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			resp := &http.Response{
				Request: &http.Request{URL: parsedURL},
				Header:  make(http.Header),
			}
			resp.Header.Set("Content-Disposition", tt.disposition)

			filename, err := guessFilename(resp)
			if err != nil {
				t.Errorf("guessFilename() returned error: %v", err)
			}

			if filename != tt.expected {
				t.Errorf("Expected filename %q, got %q", tt.expected, filename)
			}
		})
	}
}

func TestGuessFilename_InvalidCases(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		disposition string
		expectError bool
	}{
		{
			name:        "empty URL path",
			url:         "http://example.com/",
			expectError: true,
		},
		{
			name:        "root path",
			url:         "http://example.com",
			expectError: true,
		},
		{
			name:        "directory-like path",
			url:         "http://example.com/path/",
			expectError: true,
		},
		{
			name:        "null byte path handled by Go",
			url:         "http://example.com/file.txt",
			expectError: false,
		},
		{
			name:        "malformed content disposition",
			url:         "http://example.com/test.txt",
			disposition: "attachment; filename=",
			expectError: false, // Should fall back to URL
		},
		{
			name:        "empty filename in disposition",
			url:         "http://example.com/backup.zip",
			disposition: `attachment; filename=""`,
			expectError: true, // Empty filename should cause error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			resp := &http.Response{
				Request: &http.Request{URL: parsedURL},
				Header:  make(http.Header),
			}

			if tt.disposition != "" {
				resp.Header.Set("Content-Disposition", tt.disposition)
			}

			filename, err := guessFilename(resp)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if err != ErrNoFilename {
					t.Errorf("Expected ErrNoFilename, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if filename == "" {
					t.Error("Filename should not be empty when no error")
				}
			}
		})
	}
}

func TestGuessFilename_PathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "path traversal attempt",
			url:      "http://example.com/../../../etc/passwd",
			expected: "passwd",
		},
		{
			name:     "complex path traversal",
			url:      "http://example.com/path/../other/../file.txt",
			expected: "file.txt",
		},
		{
			name:     "leading slash",
			url:      "http://example.com//file.txt",
			expected: "file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			resp := &http.Response{
				Request: &http.Request{URL: parsedURL},
				Header:  make(http.Header),
			}

			filename, err := guessFilename(resp)
			if err != nil {
				t.Errorf("guessFilename() returned error: %v", err)
			}

			if filename != tt.expected {
				t.Errorf("Expected filename %q, got %q", tt.expected, filename)
			}

			// Ensure no path traversal components remain
			if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
				t.Errorf("Filename %q contains path traversal components", filename)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSetLastModified(b *testing.B) {
	setupBenchmarkDirectory(b, "util_bench_setlastmod")

	// Create a test file
	testFile := "bench_file.txt"
	file, err := os.Create(testFile)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}
	_ = file.Close()

	// Create HTTP response with Last-Modified header
	lastModTime := time.Now().UTC()
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("Last-Modified", lastModTime.Format(http.TimeFormat))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setLastModified(resp, testFile)
	}
}

func BenchmarkMkdirp(b *testing.B) {
	setupBenchmarkDirectory(b, "util_bench_mkdirp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testPath := filepath.Join("bench", "level", "deep", "path", "file.txt")
		_ = mkdirp(testPath)
	}
}

func BenchmarkGuessFilename(b *testing.B) {
	parsedURL, err := url.Parse("http://example.com/path/to/document.pdf")
	if err != nil {
		b.Fatalf("Failed to parse URL: %v", err)
	}

	resp := &http.Response{
		Request: &http.Request{URL: parsedURL},
		Header:  make(http.Header),
	}
	resp.Header.Set("Content-Disposition", `attachment; filename="document.pdf"`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = guessFilename(resp)
	}
}
