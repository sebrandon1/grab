package lib

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// newTransferWithBPS returns a transfer whose gauge reports the given rate
// using two synthetic samples one second apart, and whose byte counter is n.
func newTransferWithBPS(bps, n int64) *transfer {
	tr := newTransfer(context.Background(), nil, io.Discard, nil, nil)
	start := time.Unix(0, 0)
	tr.gauge.Sample(start, 0)
	tr.gauge.Sample(start.Add(time.Second), bps)
	atomic.StoreInt64(&tr.n, n)
	return tr
}

// completedResponse returns a Response that is already finished.
func completedResponse(start, end time.Time, err error) *Response {
	resp := &Response{
		Request: &Request{},
		Start:   start,
		End:     end,
		Done:    make(chan struct{}),
		err:     err,
	}
	close(resp.Done)
	return resp
}

func TestResponse_IsComplete(t *testing.T) {
	t.Run("in progress", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		if resp.IsComplete() {
			t.Error("IsComplete() = true before Done is closed")
		}
	})

	t.Run("complete", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		close(resp.Done)
		if !resp.IsComplete() {
			t.Error("IsComplete() = false after Done is closed")
		}
	})
}

func TestResponse_Err(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := completedResponse(time.Now(), time.Now(), nil)
		if err := resp.Err(); err != nil {
			t.Errorf("Err() = %v, want nil", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		wantErr := errors.New("boom")
		resp := completedResponse(time.Now(), time.Now(), wantErr)
		if err := resp.Err(); !errors.Is(err, wantErr) {
			t.Errorf("Err() = %v, want %v", err, wantErr)
		}
	})
}

func TestResponse_SizeAndBytesComplete(t *testing.T) {
	resp := &Response{Done: make(chan struct{})}
	atomic.StoreInt64(&resp.sizeUnsafe, 1000)
	resp.bytesResumed = 250
	resp.transfer = newTransferWithBPS(0, 750)

	if got := resp.Size(); got != 1000 {
		t.Errorf("Size() = %d, want 1000", got)
	}
	if got := resp.BytesComplete(); got != 1000 {
		t.Errorf("BytesComplete() = %d, want 1000 (250 resumed + 750 copied)", got)
	}
}

func TestResponse_Progress(t *testing.T) {
	t.Run("known size", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		atomic.StoreInt64(&resp.sizeUnsafe, 200)
		resp.transfer = newTransferWithBPS(0, 50)

		if got := resp.Progress(); got != 0.25 {
			t.Errorf("Progress() = %v, want 0.25", got)
		}
	})

	t.Run("unknown size", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		atomic.StoreInt64(&resp.sizeUnsafe, -1)
		resp.transfer = newTransferWithBPS(0, 50)

		if got := resp.Progress(); got != 0 {
			t.Errorf("Progress() with unknown size = %v, want 0", got)
		}
	})

	t.Run("zero size", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		resp.transfer = newTransferWithBPS(0, 0)

		if got := resp.Progress(); got != 0 {
			t.Errorf("Progress() with zero size = %v, want 0", got)
		}
	})
}

func TestResponse_Duration(t *testing.T) {
	t.Run("in progress", func(t *testing.T) {
		start := time.Now().Add(-50 * time.Millisecond)
		resp := &Response{Start: start, Done: make(chan struct{})}

		if got := resp.Duration(); got < 50*time.Millisecond {
			t.Errorf("Duration() = %v, want at least 50ms", got)
		}
	})

	t.Run("complete", func(t *testing.T) {
		start := time.Unix(1000, 0)
		end := start.Add(2 * time.Second)
		resp := completedResponse(start, end, nil)

		if got := resp.Duration(); got != 2*time.Second {
			t.Errorf("Duration() = %v, want 2s", got)
		}
	})
}

func TestResponse_BytesPerSecond(t *testing.T) {
	t.Run("in progress", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		resp.transfer = newTransferWithBPS(100, 100)

		if got := resp.BytesPerSecond(); got != 100 {
			t.Errorf("BytesPerSecond() = %v, want 100", got)
		}
	})

	t.Run("complete uses lifetime average", func(t *testing.T) {
		start := time.Unix(1000, 0)
		resp := completedResponse(start, start.Add(4*time.Second), nil)
		resp.transfer = newTransferWithBPS(0, 400)

		if got := resp.BytesPerSecond(); got != 100 {
			t.Errorf("BytesPerSecond() = %v, want 100 (400 bytes over 4s)", got)
		}
	})

	t.Run("no transfer", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		close(resp.Done)
		resp.Start = time.Unix(1000, 0)
		resp.End = resp.Start.Add(time.Second)

		if got := resp.BytesPerSecond(); got != 0 {
			t.Errorf("BytesPerSecond() with nil transfer = %v, want 0", got)
		}
	})
}

func TestResponse_ETA(t *testing.T) {
	t.Run("complete returns end", func(t *testing.T) {
		start := time.Unix(1000, 0)
		end := start.Add(time.Second)
		resp := completedResponse(start, end, nil)

		if got := resp.ETA(); !got.Equal(end) {
			t.Errorf("ETA() = %v, want %v", got, end)
		}
	})

	t.Run("zero bps returns zero time", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		atomic.StoreInt64(&resp.sizeUnsafe, 1000)
		resp.transfer = newTransferWithBPS(0, 0)

		if got := resp.ETA(); !got.IsZero() {
			t.Errorf("ETA() with zero BPS = %v, want zero time", got)
		}
	})

	t.Run("positive bps estimates remaining time", func(t *testing.T) {
		resp := &Response{Done: make(chan struct{})}
		atomic.StoreInt64(&resp.sizeUnsafe, 1000)
		resp.bytesResumed = 100
		resp.transfer = newTransferWithBPS(100, 100)

		// 1000 total - 200 complete = 800 remaining at 100 B/s = 8s.
		want := time.Now().Add(8 * time.Second)
		got := resp.ETA()
		if diff := got.Sub(want); diff < -2*time.Second || diff > 2*time.Second {
			t.Errorf("ETA() = %v, want approximately %v (diff %v)", got, want, diff)
		}
	})
}

func TestResponse_Wait(t *testing.T) {
	resp := &Response{Done: make(chan struct{})}
	waited := make(chan struct{})
	go func() {
		resp.Wait()
		close(waited)
	}()

	select {
	case <-waited:
		t.Fatal("Wait() returned before Done was closed")
	case <-time.After(20 * time.Millisecond):
	}

	close(resp.Done)
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("Wait() did not return after Done was closed")
	}
}

func TestResponse_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	resp := &Response{
		Done:   make(chan struct{}),
		cancel: cancel,
	}
	// Mirror grab's state machine: closing the response also closes Done.
	go func() {
		<-ctx.Done()
		resp.err = ctx.Err()
		close(resp.Done)
	}()

	if err := resp.Cancel(); !errors.Is(err, context.Canceled) {
		t.Errorf("Cancel() = %v, want context.Canceled", err)
	}
	if !resp.IsComplete() {
		t.Error("Cancel() returned before the response completed")
	}
}

func TestResponse_Open(t *testing.T) {
	t.Run("NoStore reads from memory", func(t *testing.T) {
		resp := completedResponse(time.Now(), time.Now(), nil)
		resp.Request.NoStore = true
		resp.storeBuffer.WriteString("in-memory content")

		rc, err := resp.Open()
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		defer func() { _ = rc.Close() }()

		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(got) != "in-memory content" {
			t.Errorf("Open() content = %q, want %q", got, "in-memory content")
		}
	})

	t.Run("file backed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.bin")
		if err := os.WriteFile(path, []byte("on disk"), 0o600); err != nil {
			t.Fatal(err)
		}
		resp := completedResponse(time.Now(), time.Now(), nil)
		resp.Filename = path

		rc, err := resp.Open()
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		defer func() { _ = rc.Close() }()

		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(got) != "on disk" {
			t.Errorf("Open() content = %q, want %q", got, "on disk")
		}
	})

	t.Run("returns transfer error", func(t *testing.T) {
		wantErr := errors.New("download failed")
		resp := completedResponse(time.Now(), time.Now(), wantErr)

		if _, err := resp.Open(); !errors.Is(err, wantErr) {
			t.Errorf("Open() error = %v, want %v", err, wantErr)
		}
	})
}

func TestResponse_Bytes(t *testing.T) {
	t.Run("NoStore", func(t *testing.T) {
		resp := completedResponse(time.Now(), time.Now(), nil)
		resp.Request.NoStore = true
		resp.storeBuffer.WriteString("memory bytes")

		got, err := resp.Bytes()
		if err != nil {
			t.Fatalf("Bytes() error: %v", err)
		}
		if string(got) != "memory bytes" {
			t.Errorf("Bytes() = %q, want %q", got, "memory bytes")
		}
	})

	t.Run("file backed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.bin")
		if err := os.WriteFile(path, []byte("disk bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
		resp := completedResponse(time.Now(), time.Now(), nil)
		resp.Filename = path

		got, err := resp.Bytes()
		if err != nil {
			t.Fatalf("Bytes() error: %v", err)
		}
		if string(got) != "disk bytes" {
			t.Errorf("Bytes() = %q, want %q", got, "disk bytes")
		}
	})

	t.Run("returns transfer error", func(t *testing.T) {
		wantErr := errors.New("download failed")
		resp := completedResponse(time.Now(), time.Now(), wantErr)

		if _, err := resp.Bytes(); !errors.Is(err, wantErr) {
			t.Errorf("Bytes() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		resp := completedResponse(time.Now(), time.Now(), nil)
		resp.Filename = filepath.Join(t.TempDir(), "missing.bin")

		if _, err := resp.Bytes(); err == nil {
			t.Error("Bytes() = nil error for a missing file")
		}
	})
}

func TestResponse_requestMethod(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		var resp *Response
		if got := resp.requestMethod(); got != "" {
			t.Errorf("requestMethod() = %q, want empty", got)
		}
	})

	t.Run("nil HTTPResponse", func(t *testing.T) {
		resp := &Response{}
		if got := resp.requestMethod(); got != "" {
			t.Errorf("requestMethod() = %q, want empty", got)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		resp := &Response{HTTPResponse: &http.Response{}}
		if got := resp.requestMethod(); got != "" {
			t.Errorf("requestMethod() = %q, want empty", got)
		}
	})

	t.Run("GET", func(t *testing.T) {
		resp := &Response{HTTPResponse: &http.Response{
			Request: &http.Request{Method: "GET"},
		}}
		if got := resp.requestMethod(); got != "GET" {
			t.Errorf("requestMethod() = %q, want GET", got)
		}
	})
}

func TestResponse_checksumUnsafe(t *testing.T) {
	resp := completedResponse(time.Now(), time.Now(), nil)
	resp.Request = &Request{hash: sha256.New()}
	resp.Request.NoStore = true
	resp.storeBuffer.WriteString("abc")

	want := sha256.Sum256([]byte("abc"))
	sum, err := resp.checksumUnsafe()
	if err != nil {
		t.Fatalf("checksumUnsafe() error: %v", err)
	}
	if got, wantHex := hex.EncodeToString(sum), hex.EncodeToString(want[:]); got != wantHex {
		t.Errorf("checksumUnsafe() = %s, want %s", got, wantHex)
	}
}

func TestResponse_checksumUnsafe_MissingFile(t *testing.T) {
	resp := completedResponse(time.Now(), time.Now(), nil)
	resp.Request = &Request{hash: sha256.New()}
	resp.Filename = filepath.Join(t.TempDir(), "missing.bin")

	if _, err := resp.checksumUnsafe(); err == nil {
		t.Error("checksumUnsafe() = nil error for a missing file")
	}
}
