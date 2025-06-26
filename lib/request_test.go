package lib

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"testing"
	"time"
)

func TestNewRequest_Success(t *testing.T) {
	tests := []struct {
		name        string
		dst         string
		urlStr      string
		expectedDst string
	}{
		{
			name:        "basic request",
			dst:         "file.txt",
			urlStr:      "http://example.com/download.txt",
			expectedDst: "file.txt",
		},
		{
			name:        "empty destination defaults to current dir",
			dst:         "",
			urlStr:      "http://example.com/file.zip",
			expectedDst: ".",
		},
		{
			name:        "directory destination",
			dst:         "/tmp/downloads",
			urlStr:      "https://example.com/data.json",
			expectedDst: "/tmp/downloads",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewRequest(tt.dst, tt.urlStr)

			if err != nil {
				t.Errorf("NewRequest() returned error: %v", err)
			}

			if req == nil {
				t.Fatal("NewRequest() returned nil request")
			}

			if req.Filename != tt.expectedDst {
				t.Errorf("Expected filename %q, got %q", tt.expectedDst, req.Filename)
			}

			if req.HTTPRequest == nil {
				t.Error("HTTPRequest should not be nil")
			}

			if req.HTTPRequest.Method != "GET" {
				t.Errorf("Expected GET method, got %s", req.HTTPRequest.Method)
			}

			if req.HTTPRequest.URL.String() != tt.urlStr {
				t.Errorf("Expected URL %s, got %s", tt.urlStr, req.HTTPRequest.URL.String())
			}
		})
	}
}

func TestNewRequest_InvalidURL(t *testing.T) {
	invalidURLs := []struct {
		url         string
		shouldError bool
	}{
		{"://invalid-url", true},
		{"not-a-url", false}, // This actually creates a valid relative URL
		{"", false},          // Empty string is a valid URL
		{"ftp://unsupported-scheme.com/file", false}, // FTP is a valid URL scheme
	}

	for _, test := range invalidURLs {
		t.Run("url_"+test.url, func(t *testing.T) {
			req, err := NewRequest("test.txt", test.url)

			if test.shouldError {
				if err == nil {
					t.Error("NewRequest() should return error for invalid URL")
				}
				if req != nil {
					t.Error("NewRequest() should return nil request for invalid URL")
				}
			} else {
				if err != nil {
					t.Errorf("NewRequest() should not return error for valid URL: %v", err)
				}
				if req == nil {
					t.Error("NewRequest() should return valid request")
				}
			}
		})
	}
}

func TestRequest_Context_Default(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	ctx := req.Context()

	if ctx == nil {
		t.Error("Context() should not return nil")
	}

	// Should return background context by default
	if ctx != context.Background() {
		t.Error("Default context should be background context")
	}
}

func TestRequest_WithContext_Success(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	newReq := req.WithContext(ctx)

	if newReq == nil {
		t.Fatal("WithContext() returned nil")
	}

	if newReq == req {
		t.Error("WithContext() should return a new request, not modify the original")
	}

	if newReq.Context() != ctx {
		t.Error("New request should have the provided context")
	}

	if newReq.HTTPRequest.Context() != ctx {
		t.Error("HTTPRequest should also have the provided context")
	}

	// Original request should be unchanged
	if req.Context() == ctx {
		t.Error("Original request context should not be modified")
	}
}

func TestRequest_WithContext_Nil(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("WithContext(nil) should panic")
		}
	}()

	//nolint:staticcheck // Testing that nil context panics
	req.WithContext(nil)
}

func TestRequest_WithContext_Cancelation(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	newReq := req.WithContext(ctx)

	// Cancel the context
	cancel()

	// Check that the context is canceled
	select {
	case <-newReq.Context().Done():
		// Expected - context should be canceled
	default:
		t.Error("Context should be canceled")
	}

	if newReq.Context().Err() != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", newReq.Context().Err())
	}
}

func TestRequest_URL(t *testing.T) {
	testURL := "https://example.com/path/to/file.zip?param=value"
	req, err := NewRequest("test.zip", testURL)
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	url := req.URL()

	if url == nil {
		t.Fatal("URL() returned nil")
	}

	if url.String() != testURL {
		t.Errorf("Expected URL %s, got %s", testURL, url.String())
	}

	// Test specific URL components
	if url.Scheme != "https" {
		t.Errorf("Expected scheme https, got %s", url.Scheme)
	}

	if url.Host != "example.com" {
		t.Errorf("Expected host example.com, got %s", url.Host)
	}

	if url.Path != "/path/to/file.zip" {
		t.Errorf("Expected path /path/to/file.zip, got %s", url.Path)
	}

	if url.RawQuery != "param=value" {
		t.Errorf("Expected query param=value, got %s", url.RawQuery)
	}
}

func TestRequest_SetChecksum_MD5(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// Create MD5 hash and expected checksum
	hash := md5.New()
	expectedSum := []byte("test-checksum")
	deleteOnError := true

	req.SetChecksum(hash, expectedSum, deleteOnError)

	// Verify the checksum was set (accessing private fields for testing)
	if req.hash != hash {
		t.Error("Hash was not set correctly")
	}

	if string(req.checksum) != string(expectedSum) {
		t.Errorf("Expected checksum %v, got %v", expectedSum, req.checksum)
	}

	if req.deleteOnError != deleteOnError {
		t.Errorf("Expected deleteOnError %v, got %v", deleteOnError, req.deleteOnError)
	}
}

func TestRequest_SetChecksum_SHA256(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// Create SHA256 hash and expected checksum
	hash := sha256.New()
	expectedSum := []byte("sha256-test-checksum")
	deleteOnError := false

	req.SetChecksum(hash, expectedSum, deleteOnError)

	// Verify the checksum was set
	if req.hash != hash {
		t.Error("Hash was not set correctly")
	}

	if string(req.checksum) != string(expectedSum) {
		t.Errorf("Expected checksum %v, got %v", expectedSum, req.checksum)
	}

	if req.deleteOnError != deleteOnError {
		t.Errorf("Expected deleteOnError %v, got %v", deleteOnError, req.deleteOnError)
	}
}

func TestRequest_SetChecksum_Disable(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// First set a checksum
	hash := md5.New()
	req.SetChecksum(hash, []byte("test"), true)

	// Then disable checksum validation
	req.SetChecksum(nil, nil, false)

	// Verify checksum was disabled
	if req.hash != nil {
		t.Error("Hash should be nil when disabled")
	}

	if req.checksum != nil {
		t.Error("Checksum should be nil when disabled")
	}

	if req.deleteOnError != false {
		t.Error("deleteOnError should be false when disabled")
	}
}

func TestRequest_Fields_DefaultValues(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// Check default values
	if req.Label != "" {
		t.Errorf("Expected empty Label, got %q", req.Label)
	}

	if req.Tag != nil {
		t.Errorf("Expected nil Tag, got %v", req.Tag)
	}

	if req.SkipExisting != false {
		t.Error("Expected SkipExisting to be false")
	}

	if req.NoResume != false {
		t.Error("Expected NoResume to be false")
	}

	if req.NoStore != false {
		t.Error("Expected NoStore to be false")
	}

	if req.NoCreateDirectories != false {
		t.Error("Expected NoCreateDirectories to be false")
	}

	if req.IgnoreBadStatusCodes != false {
		t.Error("Expected IgnoreBadStatusCodes to be false")
	}

	if req.IgnoreRemoteTime != false {
		t.Error("Expected IgnoreRemoteTime to be false")
	}

	if req.Size != 0 {
		t.Errorf("Expected Size to be 0, got %d", req.Size)
	}

	if req.BufferSize != 0 {
		t.Errorf("Expected BufferSize to be 0, got %d", req.BufferSize)
	}

	if req.RateLimiter != nil {
		t.Error("Expected RateLimiter to be nil")
	}

	if req.BeforeCopy != nil {
		t.Error("Expected BeforeCopy to be nil")
	}

	if req.AfterCopy != nil {
		t.Error("Expected AfterCopy to be nil")
	}
}

func TestRequest_Fields_SetValues(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// Set all configurable fields
	req.Label = "test download"
	req.Tag = "user-data"
	req.SkipExisting = true
	req.NoResume = true
	req.NoStore = true
	req.NoCreateDirectories = true
	req.IgnoreBadStatusCodes = true
	req.IgnoreRemoteTime = true
	req.Size = 1024
	req.BufferSize = 4096

	// Verify all fields were set
	if req.Label != "test download" {
		t.Errorf("Expected Label 'test download', got %q", req.Label)
	}

	if req.Tag != "user-data" {
		t.Errorf("Expected Tag 'user-data', got %v", req.Tag)
	}

	if !req.SkipExisting {
		t.Error("Expected SkipExisting to be true")
	}

	if !req.NoResume {
		t.Error("Expected NoResume to be true")
	}

	if !req.NoStore {
		t.Error("Expected NoStore to be true")
	}

	if !req.NoCreateDirectories {
		t.Error("Expected NoCreateDirectories to be true")
	}

	if !req.IgnoreBadStatusCodes {
		t.Error("Expected IgnoreBadStatusCodes to be true")
	}

	if !req.IgnoreRemoteTime {
		t.Error("Expected IgnoreRemoteTime to be true")
	}

	if req.Size != 1024 {
		t.Errorf("Expected Size 1024, got %d", req.Size)
	}

	if req.BufferSize != 4096 {
		t.Errorf("Expected BufferSize 4096, got %d", req.BufferSize)
	}
}

func TestRequest_WithContext_CopyAllFields(t *testing.T) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	// Set all fields on original request
	req.Label = "original"
	req.Tag = "original-tag"
	req.SkipExisting = true
	req.NoResume = true
	req.NoStore = true
	req.Size = 2048
	req.BufferSize = 8192

	// Add checksum
	hash := md5.New()
	req.SetChecksum(hash, []byte("checksum"), true)

	// Create new request with different context
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("key"), "value")
	newReq := req.WithContext(ctx)

	// Verify all fields were copied
	if newReq.Label != req.Label {
		t.Error("Label was not copied")
	}
	if newReq.Tag != req.Tag {
		t.Error("Tag was not copied")
	}
	if newReq.SkipExisting != req.SkipExisting {
		t.Error("SkipExisting was not copied")
	}
	if newReq.NoResume != req.NoResume {
		t.Error("NoResume was not copied")
	}
	if newReq.NoStore != req.NoStore {
		t.Error("NoStore was not copied")
	}
	if newReq.Size != req.Size {
		t.Error("Size was not copied")
	}
	if newReq.BufferSize != req.BufferSize {
		t.Error("BufferSize was not copied")
	}
	if newReq.Filename != req.Filename {
		t.Error("Filename was not copied")
	}

	// Verify checksum fields were copied
	if newReq.hash != req.hash {
		t.Error("Hash was not copied")
	}
	if string(newReq.checksum) != string(req.checksum) {
		t.Error("Checksum was not copied")
	}
	if newReq.deleteOnError != req.deleteOnError {
		t.Error("deleteOnError was not copied")
	}

	// But context should be different
	if newReq.Context() == req.Context() {
		t.Error("Context should be different")
	}
}

// Benchmark tests
func BenchmarkNewRequest(b *testing.B) {
	dst := "test.txt"
	urlStr := "http://example.com/file.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewRequest(dst, urlStr)
	}
}

func BenchmarkRequest_WithContext(b *testing.B) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		b.Fatalf("NewRequest() failed: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = req.WithContext(ctx)
	}
}

func BenchmarkRequest_URL(b *testing.B) {
	req, err := NewRequest("test.txt", "http://example.com/file.txt")
	if err != nil {
		b.Fatalf("NewRequest() failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = req.URL()
	}
}
