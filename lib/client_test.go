package lib

import (
	"context"
	"errors"
	"net/http"
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
				req, _ := NewRequest("", "https://httpbin.org/bytes/512")
				req.NoStore = true // Store in memory to avoid file system operations
				return req
			},
			expectComplete: false, // Do() returns immediately, transfer happens in background
			expectError:    false,
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			req := tt.setupRequest()

			resp := client.Do(req)

			if resp == nil {
				t.Fatal("Do() returned nil response")
			}

			if resp.Request.URL().String() != req.URL().String() {
				t.Error("Response.Request URL does not match input request URL")
			}

			if resp.Start.IsZero() {
				t.Error("Response.Start time should be set")
			}

			if resp.Done == nil {
				t.Error("Response.Done channel should be initialized")
			}

			// Wait for transfer to complete
			select {
			case <-resp.Done:
				// Transfer completed
			case <-time.After(5 * time.Second):
				t.Fatal("Transfer did not complete within timeout")
			}

			err := resp.Err()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestClient_DoChannel(t *testing.T) {
	client := &Client{
		HTTPClient: DefaultClient.HTTPClient,
		UserAgent:  "test-agent",
	}

	reqch := make(chan *Request, 2)
	respch := make(chan *Response, 2)

	req1, _ := NewRequest("", "https://httpbin.org/bytes/256")
	req1.NoStore = true
	req2, _ := NewRequest("", "https://httpbin.org/bytes/512")
	req2.NoStore = true

	reqch <- req1
	reqch <- req2
	close(reqch)

	// Run DoChannel in a goroutine
	done := make(chan struct{})
	go func() {
		client.DoChannel(reqch, respch)
		close(respch)
		close(done)
	}()

	responses := make([]*Response, 0)
	for resp := range respch {
		responses = append(responses, resp)
	}

	<-done

	if len(responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(responses))
	}

	// Verify all transfers completed
	for i, resp := range responses {
		if !resp.IsComplete() {
			t.Errorf("Response %d should be complete", i)
		}
	}
}

func TestClient_DoBatch(t *testing.T) {
	client := &Client{
		HTTPClient: DefaultClient.HTTPClient,
		UserAgent:  "test-agent",
	}

	urls := []string{
		"https://httpbin.org/bytes/256",
		"https://httpbin.org/bytes/512",
		"https://httpbin.org/bytes/1024",
	}

	var requests []*Request
	for _, url := range urls {
		req, _ := NewRequest("", url)
		req.NoStore = true
		requests = append(requests, req)
	}

	respch := client.DoBatch(2, requests...)

	responses := make([]*Response, 0)
	for resp := range respch {
		responses = append(responses, resp)
	}

	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	// Verify all transfers completed
	for i, resp := range responses {
		if !resp.IsComplete() {
			t.Errorf("Response %d should be complete", i)
		}
		if resp.Err() != nil {
			t.Errorf("Response %d error: %v", i, resp.Err())
		}
	}
}

func TestClient_DoBatch_UnlimitedWorkers(t *testing.T) {
	client := &Client{
		HTTPClient: DefaultClient.HTTPClient,
		UserAgent:  "test-agent",
	}

	req, _ := NewRequest("", "https://httpbin.org/bytes/512")
	req.NoStore = true

	// Test with workers < 1 (should create one worker per request)
	respch := client.DoBatch(0, req)

	responses := make([]*Response, 0)
	for resp := range respch {
		responses = append(responses, resp)
	}

	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
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

// Benchmark tests
func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewClient()
	}
}

func BenchmarkClient_doHTTPRequest(b *testing.B) {
	client := &Client{
		HTTPClient: DefaultClient.HTTPClient,
		UserAgent:  "bench-agent",
	}

	req, _ := http.NewRequest("GET", "https://httpbin.org/bytes/256", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.doHTTPRequest(req)
	}
}
