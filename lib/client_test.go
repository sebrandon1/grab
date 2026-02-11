package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Mock HTTP client is now in test_helpers.go

func TestNewClient(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.UserAgent != "grab" {
		t.Errorf("Expected UserAgent to be 'grab', got %q", client.UserAgent)
	}

	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}

	if client.BufferSize != 0 {
		t.Errorf("Expected BufferSize to be 0 (default), got %d", client.BufferSize)
	}
}

func TestDefaultClient(t *testing.T) {
	if DefaultClient == nil {
		t.Fatal("DefaultClient is nil")
	}

	if DefaultClient.UserAgent != "grab" {
		t.Errorf("Expected DefaultClient UserAgent to be 'grab', got %q", DefaultClient.UserAgent)
	}
}

func TestClient_Do(t *testing.T) {
	tests := []struct {
		name           string
		setupClient    func() *Client
		setupRequest   func() *Request
		expectComplete bool
		expectError    bool
		retries        int           // Number of retries for flaky network tests
		timeout        time.Duration // Per-attempt timeout
	}{
		{
			name: "successful download to memory",
			setupClient: func() *Client {
				return &Client{
					HTTPClient: DefaultClient.HTTPClient,
					UserAgent:  "test-agent",
					BufferSize: 1024,
				}
			},
			setupRequest: func() *Request {
				req, _ := NewRequest("", getWorking512ByteURL())
				req.NoStore = true // Store in memory to avoid file system operations
				return req
			},
			expectComplete: false, // Do() returns immediately, transfer happens in background
			expectError:    false,
			retries:        3,                // Retry up to 3 times for network flakiness
			timeout:        10 * time.Second, // Longer timeout for network operations
		},
		{
			name: "http client error",
			setupClient: func() *Client {
				mockClient := newMockHTTPClient()
				mockClient.addError("GET", "http://example.com/file.txt", errors.New("network error"))

				return &Client{
					HTTPClient: mockClient,
					UserAgent:  "test-agent",
				}
			},
			setupRequest: func() *Request {
				req, _ := NewRequest("", "http://example.com/file.txt")
				req.NoStore = true
				return req
			},
			expectComplete: false,
			expectError:    true,
			retries:        1, // Mock tests don't need retries
			timeout:        5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lastErr error
			var success bool

			// Retry logic for flaky network tests
			for attempt := 1; attempt <= tt.retries; attempt++ {
				client := tt.setupClient()
				req := tt.setupRequest()

				resp := client.Do(req)

				if resp == nil {
					lastErr = errors.New("Do() returned nil response")
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond) // Progressive backoff
						continue
					}
					t.Fatal(lastErr)
				}

				if resp.Request.URL().String() != req.URL().String() {
					lastErr = errors.New("Response.Request URL does not match input request URL")
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
						continue
					}
					t.Error(lastErr)
					return
				}

				if resp.Start.IsZero() {
					lastErr = errors.New("Response.Start time should be set")
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
						continue
					}
					t.Error(lastErr)
					return
				}

				if resp.Done == nil {
					lastErr = errors.New("Response.Done channel should be initialized")
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
						continue
					}
					t.Error(lastErr)
					return
				}

				// Wait for transfer to complete with per-attempt timeout
				transferStart := time.Now()
				select {
				case <-resp.Done:
					// Transfer completed successfully
					success = true
				case <-time.After(tt.timeout):
					lastErr = fmt.Errorf("Transfer did not complete within %v timeout (attempt %d/%d)", tt.timeout, attempt, tt.retries)
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
						continue
					}
					t.Fatal(lastErr)
				}

				transferDuration := time.Since(transferStart)
				t.Logf("Transfer completed in %v (attempt %d/%d)", transferDuration, attempt, tt.retries)

				err := resp.Err()
				if tt.expectError && err == nil {
					lastErr = errors.New("Expected error but got none")
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
						continue
					}
					t.Error(lastErr)
					return
				}
				if !tt.expectError && err != nil {
					lastErr = fmt.Errorf("Unexpected error: %v", err)
					if attempt < tt.retries {
						t.Logf("Attempt %d/%d failed: %v, retrying...", attempt, tt.retries, lastErr)
						time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
						continue
					}
					t.Error(lastErr)
					return
				}

				// If we reach here, the test passed
				success = true
				if attempt > 1 {
					t.Logf("Test succeeded on attempt %d/%d", attempt, tt.retries)
				}
				break
			}

			if !success && lastErr != nil {
				t.Fatalf("Test failed after %d attempts, last error: %v", tt.retries, lastErr)
			}
		})
	}
}

func TestClient_DoChannel(t *testing.T) {
	// Retry logic for network-dependent test
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		success := t.Run(fmt.Sprintf("attempt_%d", attempt), func(t *testing.T) {
			client := &Client{
				HTTPClient: DefaultClient.HTTPClient,
				UserAgent:  "test-agent",
			}

			reqch := make(chan *Request, 2)
			respch := make(chan *Response, 2)

			req1, _ := NewRequest("", getWorking256ByteURL())
			req1.NoStore = true
			req2, _ := NewRequest("", getWorking512ByteURL())
			req2.NoStore = true

			reqch <- req1
			reqch <- req2
			close(reqch)

			// Run DoChannel in a goroutine
			done := make(chan struct{})
			go func() {
				client.DoChannel(context.Background(), reqch, respch)
				close(respch)
				close(done)
			}()

			// Add timeout for the entire operation
			timeout := time.After(15 * time.Second)
			responses := make([]*Response, 0)

		responseLoop:
			for {
				select {
				case resp, ok := <-respch:
					if !ok {
						break responseLoop
					}
					responses = append(responses, resp)
				case <-timeout:
					lastErr = errors.New("DoChannel test timed out after 15 seconds")
					t.Error(lastErr)
					return
				}
			}

			<-done

			if len(responses) != 2 {
				lastErr = fmt.Errorf("Expected 2 responses, got %d", len(responses))
				t.Error(lastErr)
				return
			}

			// Verify all transfers completed
			for i, resp := range responses {
				if !resp.IsComplete() {
					lastErr = fmt.Errorf("Response %d should be complete", i)
					t.Error(lastErr)
					return
				}
				if resp.Err() != nil {
					lastErr = fmt.Errorf("Response %d has error: %v", i, resp.Err())
					t.Error(lastErr)
					return
				}
			}
		})

		if success {
			if attempt > 1 {
				t.Logf("DoChannel test succeeded on attempt %d/%d", attempt, maxRetries)
			}
			return // Test passed, exit retry loop
		}

		if attempt < maxRetries {
			t.Logf("DoChannel test attempt %d/%d failed, retrying...", attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}

	if lastErr != nil {
		t.Fatalf("DoChannel test failed after %d attempts, last error: %v", maxRetries, lastErr)
	}
}

func TestClient_DoBatch(t *testing.T) {
	// Retry logic for network-dependent test
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		success := t.Run(fmt.Sprintf("attempt_%d", attempt), func(t *testing.T) {
			client := &Client{
				HTTPClient: DefaultClient.HTTPClient,
				UserAgent:  "test-agent",
			}

			urls := []string{
				getWorking256ByteURL(),
				getWorking512ByteURL(),
				getWorking1024ByteURL(),
			}

			var requests []*Request
			for _, url := range urls {
				req, _ := NewRequest("", url)
				req.NoStore = true
				requests = append(requests, req)
			}

			respch := client.DoBatch(context.Background(), 2, requests...)

			// Add timeout for the entire batch operation
			timeout := time.After(20 * time.Second) // Longer timeout for batch operations
			responses := make([]*Response, 0)

		batchLoop:
			for {
				select {
				case resp, ok := <-respch:
					if !ok {
						break batchLoop
					}
					responses = append(responses, resp)
				case <-timeout:
					lastErr = errors.New("DoBatch test timed out after 20 seconds")
					t.Error(lastErr)
					return
				}
			}

			if len(responses) != 3 {
				lastErr = fmt.Errorf("Expected 3 responses, got %d", len(responses))
				t.Error(lastErr)
				return
			}

			// Verify all transfers completed
			for i, resp := range responses {
				if !resp.IsComplete() {
					lastErr = fmt.Errorf("Response %d should be complete", i)
					t.Error(lastErr)
					return
				}
				if resp.Err() != nil {
					lastErr = fmt.Errorf("Response %d error: %v", i, resp.Err())
					t.Error(lastErr)
					return
				}
			}
		})

		if success {
			if attempt > 1 {
				t.Logf("DoBatch test succeeded on attempt %d/%d", attempt, maxRetries)
			}
			return // Test passed, exit retry loop
		}

		if attempt < maxRetries {
			t.Logf("DoBatch test attempt %d/%d failed, retrying...", attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}

	if lastErr != nil {
		t.Fatalf("DoBatch test failed after %d attempts, last error: %v", maxRetries, lastErr)
	}
}

func TestClient_DoBatch_UnlimitedWorkers(t *testing.T) {
	// Retry logic for network-dependent test
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		success := t.Run(fmt.Sprintf("attempt_%d", attempt), func(t *testing.T) {
			client := &Client{
				HTTPClient: DefaultClient.HTTPClient,
				UserAgent:  "test-agent",
			}

			req, _ := NewRequest("", getWorking512ByteURL())
			req.NoStore = true

			// Test with workers < 1 (should create one worker per request)
			respch := client.DoBatch(context.Background(), 0, req)

			// Add timeout for the operation
			timeout := time.After(10 * time.Second)
			responses := make([]*Response, 0)

		unlimitedLoop:
			for {
				select {
				case resp, ok := <-respch:
					if !ok {
						break unlimitedLoop
					}
					responses = append(responses, resp)
				case <-timeout:
					lastErr = errors.New("DoBatch_UnlimitedWorkers test timed out after 10 seconds")
					t.Error(lastErr)
					return
				}
			}

			if len(responses) != 1 {
				lastErr = fmt.Errorf("Expected 1 response, got %d", len(responses))
				t.Error(lastErr)
				return
			}

			// Verify the transfer completed successfully
			if !responses[0].IsComplete() {
				lastErr = errors.New("Response should be complete")
				t.Error(lastErr)
				return
			}
			if responses[0].Err() != nil {
				lastErr = fmt.Errorf("Response error: %v", responses[0].Err())
				t.Error(lastErr)
				return
			}
		})

		if success {
			if attempt > 1 {
				t.Logf("DoBatch_UnlimitedWorkers test succeeded on attempt %d/%d", attempt, maxRetries)
			}
			return // Test passed, exit retry loop
		}

		if attempt < maxRetries {
			t.Logf("DoBatch_UnlimitedWorkers test attempt %d/%d failed, retrying...", attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}

	if lastErr != nil {
		t.Fatalf("DoBatch_UnlimitedWorkers test failed after %d attempts, last error: %v", maxRetries, lastErr)
	}
}

func TestClient_doHTTPRequest(t *testing.T) {
	tests := []struct {
		name           string
		userAgent      string
		requestHeaders map[string]string
		expectUA       string
	}{
		{
			name:      "set user agent when none exists",
			userAgent: "test-agent",
			expectUA:  "test-agent",
		},
		{
			name:      "don't override existing user agent",
			userAgent: "test-agent",
			requestHeaders: map[string]string{
				"User-Agent": "existing-agent",
			},
			expectUA: "existing-agent",
		},
		{
			name:      "empty user agent",
			userAgent: "",
			expectUA:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockHTTPClient()
			client := &Client{
				HTTPClient: mockClient,
				UserAgent:  tt.userAgent,
			}

			req, _ := http.NewRequest("GET", "http://example.com/test", nil)
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			_, err := client.doHTTPRequest(req)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check that the request was made with correct User-Agent
			requests := mockClient.getRequests()
			if len(requests) != 1 {
				t.Fatalf("Expected 1 request, got %d", len(requests))
			}

			actualUA := requests[0].Header.Get("User-Agent")
			if actualUA != tt.expectUA {
				t.Errorf("Expected User-Agent %q, got %q", tt.expectUA, actualUA)
			}
		})
	}
}

func TestClient_run(t *testing.T) {
	client := NewClient()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp := &Response{
		ctx:    ctx,
		cancel: cancel,
		Done:   make(chan struct{}),
	}

	callCount := 0
	var testStateFunc stateFunc
	testStateFunc = func(r *Response) stateFunc {
		callCount++
		if callCount >= 3 {
			// End the state machine after 3 calls
			return nil
		}
		return testStateFunc
	}

	client.run(resp, testStateFunc)

	if callCount != 3 {
		t.Errorf("Expected stateFunc to be called 3 times, got %d", callCount)
	}
}

func TestClient_run_ContextCanceled(t *testing.T) {
	client := NewClient()

	ctx, cancel := context.WithCancel(context.Background())
	resp := &Response{
		ctx:    ctx,
		cancel: cancel,
		Done:   make(chan struct{}),
	}

	// Cancel context immediately
	cancel()

	callCount := 0
	var testStateFunc stateFunc
	testStateFunc = func(r *Response) stateFunc {
		callCount++
		// This should not be called because context is canceled
		return testStateFunc
	}

	client.run(resp, testStateFunc)

	// The run should terminate due to canceled context
	if resp.err == nil {
		t.Error("Expected error due to canceled context")
	}
	if resp.err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", resp.err)
	}
}

// Test helper function to create a minimal valid response for state testing
func createTestResponse(t *testing.T, client *Client) *Response {
	req, err := NewRequest("", "http://example.com/test.txt")
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.NoStore = true

	ctx, cancel := context.WithCancel(context.Background())
	resp := &Response{
		Request:    req,
		Start:      time.Now(),
		Done:       make(chan struct{}),
		Filename:   req.Filename,
		ctx:        ctx,
		cancel:     cancel,
		bufferSize: 1024,
	}

	return resp
}

func TestClient_statFileInfo(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name          string
		setupResponse func() *Response
		expectedNext  string // name of expected next state function
	}{
		{
			name: "NoStore request",
			setupResponse: func() *Response {
				resp := createTestResponse(t, client)
				resp.Request.NoStore = true
				return resp
			},
			expectedNext: "headRequest",
		},
		{
			name: "Empty filename",
			setupResponse: func() *Response {
				resp := createTestResponse(t, client)
				resp.Filename = ""
				return resp
			},
			expectedNext: "headRequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResponse()
			nextFunc := client.statFileInfo(resp)

			// We can't easily compare function pointers, so we'll test the behavior
			// For now, just ensure we got a function back
			if nextFunc == nil && tt.expectedNext != "nil" {
				t.Error("Expected next state function, got nil")
			}
			if nextFunc != nil && tt.expectedNext == "nil" {
				t.Error("Expected nil next state function, got non-nil")
			}
		})
	}
}

func TestClient_DoChannel_ContextCanceled(t *testing.T) {
	client := &Client{
		HTTPClient: DefaultClient.HTTPClient,
		UserAgent:  "test-agent",
	}

	// Create 5 requests but cancel after sending 2
	reqch := make(chan *Request, 5)
	respch := make(chan *Response, 5)

	for i := 0; i < 5; i++ {
		req, _ := NewRequest("", getWorking256ByteURL())
		req.NoStore = true
		reqch <- req
	}
	close(reqch)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		client.DoChannel(ctx, reqch, respch)
		close(done)
	}()

	// Cancel the context immediately
	cancel()

	// Wait for DoChannel to return
	select {
	case <-done:
		// DoChannel returned due to context cancellation
	case <-time.After(10 * time.Second):
		t.Fatal("DoChannel did not return after context cancellation")
	}
}

func TestClient_getRequest_ContentRange(t *testing.T) {
	tests := []struct {
		name         string
		contentRange string
		bytesResumed int64
		didResume    bool
		statusCode   int
		expectError  bool
	}{
		{
			name:         "matching content range",
			contentRange: "bytes 100-199/200",
			bytesResumed: 100,
			didResume:    true,
			statusCode:   http.StatusPartialContent,
			expectError:  false,
		},
		{
			name:         "mismatched content range",
			contentRange: "bytes 50-199/200",
			bytesResumed: 100,
			didResume:    true,
			statusCode:   http.StatusPartialContent,
			expectError:  true,
		},
		{
			name:         "missing content range header",
			contentRange: "",
			bytesResumed: 100,
			didResume:    true,
			statusCode:   http.StatusPartialContent,
			expectError:  false,
		},
		{
			name:         "not a resumed download",
			contentRange: "bytes 50-199/200",
			bytesResumed: 100,
			didResume:    false,
			statusCode:   http.StatusOK,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testURL := "http://example.com/file.txt"

			// Build mock HTTP response
			headers := make(http.Header)
			if tt.contentRange != "" {
				headers.Set("Content-Range", tt.contentRange)
			}
			body := "test content"
			mockResp := &http.Response{
				Status:        http.StatusText(tt.statusCode),
				StatusCode:    tt.statusCode,
				Proto:         "HTTP/1.1",
				Body:          io.NopCloser(strings.NewReader(body)),
				ContentLength: int64(len(body)),
				Header:        headers,
			}

			mockClient := newMockHTTPClient()
			mockClient.addResponse("GET", testURL, mockResp)

			client := &Client{
				HTTPClient: mockClient,
				UserAgent:  "test-agent",
			}

			req, _ := NewRequest("", testURL)
			req.NoStore = true

			ctx, cancel := context.WithCancel(context.Background())
			resp := &Response{
				Request:      req,
				Start:        time.Now(),
				Done:         make(chan struct{}),
				Filename:     req.Filename,
				ctx:          ctx,
				cancel:       cancel,
				bufferSize:   1024,
				DidResume:    tt.didResume,
				bytesResumed: tt.bytesResumed,
			}

			nextFunc := client.getRequest(resp)
			_ = nextFunc

			if tt.expectError && resp.err == nil {
				t.Error("Expected error for mismatched Content-Range, got nil")
			}
			if !tt.expectError && resp.err != nil {
				t.Errorf("Unexpected error: %v", resp.err)
			}
		})
	}
}

func TestClient_NoStore_NoFilename(t *testing.T) {
	// Test that NoStore requests succeed even when the URL has no parseable filename
	mockClient := newMockHTTPClient()
	testURL := "http://example.com/"
	content := "hello world"
	mockClient.addResponse("GET", testURL, createSuccessResponse(content))

	client := &Client{
		HTTPClient: mockClient,
		UserAgent:  "test-agent",
	}

	req, err := NewRequest("", testURL)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.NoStore = true

	resp := client.Do(req)
	if err := resp.Err(); err != nil {
		t.Fatalf("Expected no error for NoStore request with no filename, got: %v", err)
	}

	data, err := resp.Bytes()
	if err != nil {
		t.Fatalf("Failed to read response bytes: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

// Benchmark tests
func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewClient()
	}
}

func BenchmarkClient_doHTTPRequest(b *testing.B) {
	// Use a mock client for benchmarks to avoid network variability
	mockClient := newMockHTTPClient()
	mockClient.addResponse("GET", "http://example.com/benchmark", createSuccessResponse("benchmark content"))

	client := &Client{
		HTTPClient: mockClient,
		UserAgent:  "bench-agent",
	}

	req, _ := http.NewRequest("GET", "http://example.com/benchmark", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.doHTTPRequest(req)
	}
}
