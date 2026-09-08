package lib

import (
	"errors"
	"os"
	"testing"
	"time"
)

var errTestHook = errors.New("hook error")

func waitForResponse(t *testing.T, resp *Response) {
	t.Helper()
	select {
	case <-resp.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("transfer did not complete within 5s")
	}
}

func TestBeforeCopy_IsCalled(t *testing.T) {
	called := false

	client := &Client{HTTPClient: newMockHTTPClient(), UserAgent: "test"}
	req, _ := NewRequest("", "http://example.com/file.txt")
	req.NoStore = true
	req.BeforeCopy = func(resp *Response) error {
		called = true
		return nil
	}

	waitForResponse(t, client.Do(req))

	if !called {
		t.Error("BeforeCopy hook was not called")
	}
}

func TestBeforeCopy_ErrorCancelsDownload(t *testing.T) {
	dir := t.TempDir()

	client := &Client{HTTPClient: newMockHTTPClient(), UserAgent: "test"}
	req, _ := NewRequest(dir, "http://example.com/file.txt")
	req.BeforeCopy = func(resp *Response) error {
		return errTestHook
	}

	resp := client.Do(req)
	waitForResponse(t, resp)

	if err := resp.Err(); err == nil {
		t.Fatal("expected error from BeforeCopy but got nil")
	} else if !errors.Is(err, errTestHook) {
		t.Errorf("expected errTestHook, got %v", err)
	}

	// The file is opened before BeforeCopy is called, so an empty file may
	// exist, but no data should have been transferred into it.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > 0 {
			t.Errorf("BeforeCopy error: file %q should be empty, got %d bytes", e.Name(), info.Size())
		}
	}
}

func TestAfterCopy_IsCalled(t *testing.T) {
	called := false

	client := &Client{HTTPClient: newMockHTTPClient(), UserAgent: "test"}
	req, _ := NewRequest("", "http://example.com/file.txt")
	req.NoStore = true
	req.AfterCopy = func(resp *Response) error {
		called = true
		return nil
	}

	waitForResponse(t, client.Do(req))

	if !called {
		t.Error("AfterCopy hook was not called")
	}
}

func TestAfterCopy_ErrorPropagates(t *testing.T) {
	client := &Client{HTTPClient: newMockHTTPClient(), UserAgent: "test"}
	req, _ := NewRequest("", "http://example.com/file.txt")
	req.NoStore = true
	req.AfterCopy = func(resp *Response) error {
		return errTestHook
	}

	resp := client.Do(req)
	waitForResponse(t, resp)

	if err := resp.Err(); err == nil {
		t.Fatal("expected error from AfterCopy but got nil")
	} else if !errors.Is(err, errTestHook) {
		t.Errorf("expected errTestHook, got %v", err)
	}
}
