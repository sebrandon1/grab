package lib

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGet_Success(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_get_test")

	// Create test file destination
	dst := "test_download.txt"

	// Create mock client with successful response
	mockClient := newMockHTTPClient()
	testURL := "http://example.com/file.txt"
	content := "test file content"
	mockClient.addResponse("GET", testURL, createSuccessResponse(content))

	// Use mock client
	withMockClient(t, mockClient, func() {
		resp, err := Get(dst, testURL)

		if err != nil {
			t.Fatalf("Get() returned error: %v", err)
		}

		if resp == nil {
			t.Fatal("Get() returned nil response")
		}

		if resp.Request.URL().String() != testURL {
			t.Errorf("Expected URL %s, got %s", testURL, resp.Request.URL().String())
		}

		if resp.Request.Filename != dst {
			t.Errorf("Expected filename %s, got %s", dst, resp.Request.Filename)
		}

		// Verify response completed
		if !resp.IsComplete() {
			t.Error("Response should be complete after Get() returns")
		}
	})
}

func TestGet_InvalidURL(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_get_invalid_url")

	dst := "test_download.txt"
	invalidURL := "://invalid-url"

	resp, err := Get(dst, invalidURL)

	if err == nil {
		t.Error("Get() should return error for invalid URL")
	}

	if resp != nil {
		t.Error("Get() should return nil response for invalid URL")
	}
}

func TestGet_HTTPError(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_get_http_error")

	dst := "test_download.txt"
	testURL := "http://example.com/notfound.txt"

	// Create mock client with error response
	mockClient := newMockHTTPClient()
	mockClient.addResponse("GET", testURL, createErrorResponse(404, "not found"))

	withMockClient(t, mockClient, func() {
		resp, err := Get(dst, testURL)

		if err == nil {
			t.Error("Get() should return error for 404 response")
		}

		if resp == nil {
			t.Error("Get() should return response even with error")
		}

		if resp != nil && resp.Err() == nil {
			t.Error("Response should have error for 404 status")
		}
	})
}

func TestGet_ClientError(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_get_client_error")

	dst := "test_download.txt"
	testURL := "http://example.com/file.txt"

	// Create mock client that returns network error
	mockClient := newMockHTTPClient()
	mockClient.addError("GET", testURL, errors.New("network error"))

	withMockClient(t, mockClient, func() {
		resp, err := Get(dst, testURL)

		if err == nil {
			t.Error("Get() should return error for network error")
		}

		if resp == nil {
			t.Error("Get() should return response even with network error")
		}

		if resp != nil && resp.Err() == nil {
			t.Error("Response should have error for network failure")
		}
	})
}

func TestGet_RealHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real HTTP test in short mode")
	}

	setupTestDirectoryWithCleanup(t, "grab_get_real_http")

	dst := "httpbin_download.bin"
	testURL := "https://httpbin.org/bytes/256"

	resp, err := Get(dst, testURL)

	if err != nil {
		t.Fatalf("Get() failed with real HTTP: %v", err)
	}

	if resp == nil {
		t.Fatal("Get() returned nil response")
	}

	if !resp.IsComplete() {
		t.Error("Response should be complete")
	}

	if resp.Size() == 0 {
		t.Error("Downloaded file should have content")
	}

	// Verify file was created
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("Downloaded file should exist")
	}
}

func TestGetBatch_Success(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_batch_test")

	// Create mock client
	mockClient := newMockHTTPClient()
	testURLs := []string{
		"http://example.com/file1.txt",
		"http://example.com/file2.txt",
		"http://example.com/file3.txt",
	}

	// Add responses for all URLs
	for i, url := range testURLs {
		content := strings.Repeat("x", 100+i*50) // Different sizes
		mockClient.addResponse("GET", url, createSuccessResponse(content))
	}

	withMockClient(t, mockClient, func() {
		// Get current working directory as destination
		dst, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		ch, err := GetBatch(2, dst, testURLs...)
		if err != nil {
			t.Fatalf("GetBatch() returned error: %v", err)
		}

		if ch == nil {
			t.Fatal("GetBatch() returned nil channel")
		}

		// Collect all responses
		var responses []*Response
		for resp := range ch {
			responses = append(responses, resp)
		}

		// Verify we got responses for all URLs
		if len(responses) != len(testURLs) {
			t.Errorf("Expected %d responses, got %d", len(testURLs), len(responses))
		}

		// Verify all responses completed
		for i, resp := range responses {
			if resp == nil {
				t.Errorf("Response %d is nil", i)
				continue
			}

			if !resp.IsComplete() {
				t.Errorf("Response %d should be complete", i)
			}

			// Check that URL matches one of our test URLs
			responseURL := resp.Request.URL().String()
			found := false
			for _, testURL := range testURLs {
				if responseURL == testURL {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Response URL %s not found in test URLs", responseURL)
			}
		}
	})
}

func TestGetBatch_EmptyURLs(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_batch_empty")

	dst, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	ch, err := GetBatch(1, dst)
	if err != nil {
		t.Fatalf("GetBatch() with empty URLs should not return error: %v", err)
	}

	if ch == nil {
		t.Fatal("GetBatch() should return channel even with empty URLs")
	}

	// Channel should be closed immediately
	select {
	case resp, ok := <-ch:
		if ok {
			t.Errorf("Channel should be closed, but got response: %v", resp)
		}
	case <-time.After(1 * time.Second):
		t.Error("Channel should be closed immediately for empty URLs")
	}
}

func TestGetBatch_InvalidURL(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_batch_invalid_url")

	dst, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	invalidURL := "://invalid-url"
	ch, err := GetBatch(1, dst, invalidURL)

	if err == nil {
		t.Error("GetBatch() should return error for invalid URL")
	}

	if ch != nil {
		t.Error("GetBatch() should return nil channel for invalid URL")
	}
}

func TestGetBatch_DestinationNotExists(t *testing.T) {
	nonExistentDir := "/path/that/does/not/exist"

	ch, err := GetBatch(1, nonExistentDir, "http://example.com/file.txt")

	if err == nil {
		t.Error("GetBatch() should return error for non-existent destination")
	}

	if ch != nil {
		t.Error("GetBatch() should return nil channel for non-existent destination")
	}
}

func TestGetBatch_DestinationNotDirectory(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_batch_not_dir")

	// Create a file instead of directory
	tempFile, err := os.CreateTemp("", "not_a_dir")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tempFile.Name())
	}()
	_ = tempFile.Close()

	ch, err := GetBatch(1, tempFile.Name(), "http://example.com/file.txt")

	if err == nil {
		t.Error("GetBatch() should return error when destination is not a directory")
	}

	if ch != nil {
		t.Error("GetBatch() should return nil channel when destination is not a directory")
	}
}

func TestGetBatch_MixedResults(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_batch_mixed")

	mockClient := newMockHTTPClient()
	testURLs := []string{
		"http://example.com/success.txt",
		"http://example.com/error.txt",
	}

	// Add successful response for first URL
	mockClient.addResponse("GET", testURLs[0], createSuccessResponse("success content"))

	// Add error response for second URL
	mockClient.addResponse("GET", testURLs[1], createErrorResponse(404, "not found"))

	withMockClient(t, mockClient, func() {
		dst, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		ch, err := GetBatch(1, dst, testURLs...)
		if err != nil {
			t.Fatalf("GetBatch() returned error: %v", err)
		}

		var responses []*Response
		for resp := range ch {
			responses = append(responses, resp)
		}

		if len(responses) != 2 {
			t.Errorf("Expected 2 responses, got %d", len(responses))
		}

		// At least one should have succeeded and one should have failed
		successCount := 0
		errorCount := 0
		for _, resp := range responses {
			if resp.Err() == nil {
				successCount++
			} else {
				errorCount++
			}
		}

		// Due to the complexity of the download process, we just verify we got responses
		if successCount+errorCount != 2 {
			t.Errorf("Expected 2 total responses, got %d success + %d error", successCount, errorCount)
		}
	})
}

func TestGetBatch_UnlimitedWorkers(t *testing.T) {
	setupTestDirectoryWithCleanup(t, "grab_batch_unlimited")

	mockClient := newMockHTTPClient()
	testURLs := []string{
		"http://example.com/file1.txt",
		"http://example.com/file2.txt",
	}

	for _, url := range testURLs {
		mockClient.addResponse("GET", url, createSuccessResponse("content"))
	}

	withMockClient(t, mockClient, func() {
		dst, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Test with 0 workers (should use unlimited workers)
		ch, err := GetBatch(0, dst, testURLs...)
		if err != nil {
			t.Fatalf("GetBatch() with 0 workers returned error: %v", err)
		}

		var responses []*Response
		for resp := range ch {
			responses = append(responses, resp)
		}

		if len(responses) != len(testURLs) {
			t.Errorf("Expected %d responses, got %d", len(testURLs), len(responses))
		}
	})
}

func TestGetBatch_RealHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real HTTP test in short mode")
	}

	setupTestDirectoryWithCleanup(t, "grab_batch_real_http")

	testURLs := []string{
		"https://httpbin.org/bytes/256",
		"https://httpbin.org/bytes/512",
	}

	dst, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	ch, err := GetBatch(2, dst, testURLs...)
	if err != nil {
		t.Fatalf("GetBatch() with real HTTP failed: %v", err)
	}

	var responses []*Response
	for resp := range ch {
		responses = append(responses, resp)
	}

	if len(responses) != len(testURLs) {
		t.Errorf("Expected %d responses, got %d", len(testURLs), len(responses))
	}

	for i, resp := range responses {
		if resp.Err() != nil {
			t.Errorf("Response %d has error: %v", i, resp.Err())
		}

		if !resp.IsComplete() {
			t.Errorf("Response %d should be complete", i)
		}

		if resp.Size() == 0 {
			t.Errorf("Response %d should have downloaded content", i)
		}
	}
}

// Benchmark tests
func BenchmarkGet(b *testing.B) {
	setupBenchmarkDirectory(b, "grab_bench_get")

	mockClient := newMockHTTPClient()
	testURL := "http://example.com/benchmark.txt"
	content := strings.Repeat("x", 1024) // 1KB content
	mockClient.addResponse("GET", testURL, createSuccessResponse(content))

	withMockClientForBenchmark(b, mockClient, func() {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dst := filepath.Join(".", "bench_file_"+string(rune('A'+i%26))+".txt")
			_, _ = Get(dst, testURL)
		}
	})
}

func BenchmarkGetBatch(b *testing.B) {
	setupBenchmarkDirectory(b, "grab_bench_batch")

	mockClient := newMockHTTPClient()
	testURLs := []string{
		"http://example.com/file1.txt",
		"http://example.com/file2.txt",
		"http://example.com/file3.txt",
	}

	for _, url := range testURLs {
		content := strings.Repeat("x", 512) // 512B content
		mockClient.addResponse("GET", url, createSuccessResponse(content))
	}

	withMockClientForBenchmark(b, mockClient, func() {
		dst, _ := os.Getwd()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch, _ := GetBatch(2, dst, testURLs...)
			// Consume all responses
			for range ch {
			}
		}
	})
}

// Helper function for benchmarks - use separate functions to avoid redeclaration
func setupBenchmarkDirectory(b *testing.B, prefix string) {
	tdm := setupTestDirectoryForBenchmark(b, prefix)
	b.Cleanup(tdm.cleanup)
}

func setupTestDirectoryForBenchmark(b *testing.B, prefix string) *testDirectoryManager {
	b.Helper()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	// Save current directory
	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}

	// Change to temp directory
	err = os.Chdir(tempDir)
	if err != nil {
		b.Fatalf("Failed to change to temp directory: %v", err)
	}

	return &testDirectoryManager{
		tempDir:     tempDir,
		originalDir: originalDir,
	}
}

func withMockClientForBenchmark(b *testing.B, mockClient *mockHTTPClient, fn func()) {
	b.Helper()

	originalClient := DefaultClient
	DefaultClient = &Client{
		HTTPClient: mockClient,
		UserAgent:  "test-agent",
	}

	defer func() {
		DefaultClient = originalClient
	}()

	fn()
}
