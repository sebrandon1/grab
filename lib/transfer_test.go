package lib

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Mock rate limiter for testing
type mockRateLimiter struct {
	waitCalled   int64
	waitError    error
	waitDuration time.Duration
	shouldWait   bool
}

func (m *mockRateLimiter) WaitN(ctx context.Context, n int) error {
	atomic.AddInt64(&m.waitCalled, 1)
	if m.shouldWait && m.waitDuration > 0 {
		select {
		case <-time.After(m.waitDuration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.waitError
}

func (m *mockRateLimiter) getWaitCalled() int64 {
	return atomic.LoadInt64(&m.waitCalled)
}

// Mock reader that can simulate errors
type mockReader struct {
	data      []byte
	pos       int
	err       error
	readDelay time.Duration
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.readDelay > 0 {
		time.Sleep(m.readDelay)
	}

	if m.err != nil {
		return 0, m.err
	}

	if m.pos >= len(m.data) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

// Mock writer that can simulate errors
type mockWriter struct {
	buf        bytes.Buffer
	writeErr   error
	shortWrite bool
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}

	if m.shortWrite {
		// Simulate short write
		if len(p) > 1 {
			n = len(p) - 1
			m.buf.Write(p[:n])
			return n, nil
		}
	}

	return m.buf.Write(p)
}

func (m *mockWriter) Bytes() []byte {
	return m.buf.Bytes()
}

func TestNewTransfer(t *testing.T) {
	ctx := context.Background()
	rateLimiter := &mockRateLimiter{}
	dst := &bytes.Buffer{}
	src := strings.NewReader("test data")
	buf := make([]byte, 1024)

	transfer := newTransfer(ctx, rateLimiter, dst, src, buf)

	if transfer == nil {
		t.Fatal("newTransfer() returned nil")
	}

	if transfer.ctx != ctx {
		t.Error("Context was not set correctly")
	}

	if transfer.lim != rateLimiter {
		t.Error("Rate limiter was not set correctly")
	}

	if transfer.w != dst {
		t.Error("Writer was not set correctly")
	}

	if transfer.r != src {
		t.Error("Reader was not set correctly")
	}

	if &transfer.b[0] != &buf[0] {
		t.Error("Buffer was not set correctly")
	}

	if transfer.n != 0 {
		t.Error("Initial bytes transferred should be 0")
	}
}

func TestTransfer_Copy_Success(t *testing.T) {
	ctx := context.Background()
	testData := "Hello, World! This is test data for transfer."
	src := strings.NewReader(testData)
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()
	if err != nil {
		t.Errorf("copy() returned error: %v", err)
	}

	if written != int64(len(testData)) {
		t.Errorf("Expected %d bytes written, got %d", len(testData), written)
	}

	if dst.String() != testData {
		t.Errorf("Expected %q, got %q", testData, dst.String())
	}

	if transfer.N() != written {
		t.Errorf("N() should return %d, got %d", written, transfer.N())
	}
}

func TestTransfer_Copy_WithBuffer(t *testing.T) {
	ctx := context.Background()
	testData := strings.Repeat("A", 1000) // 1KB of data
	src := strings.NewReader(testData)
	dst := &bytes.Buffer{}
	buf := make([]byte, 100) // Small buffer to force multiple reads

	transfer := newTransfer(ctx, nil, dst, src, buf)

	written, err := transfer.copy()
	if err != nil {
		t.Errorf("copy() returned error: %v", err)
	}

	if written != int64(len(testData)) {
		t.Errorf("Expected %d bytes written, got %d", len(testData), written)
	}

	if dst.String() != testData {
		t.Error("Data was not copied correctly")
	}
}

func TestTransfer_Copy_WithRateLimiter(t *testing.T) {
	ctx := context.Background()
	testData := "Rate limited data"
	src := strings.NewReader(testData)
	dst := &bytes.Buffer{}
	rateLimiter := &mockRateLimiter{}

	transfer := newTransfer(ctx, rateLimiter, dst, src, nil)

	written, err := transfer.copy()
	if err != nil {
		t.Errorf("copy() returned error: %v", err)
	}

	if written != int64(len(testData)) {
		t.Errorf("Expected %d bytes written, got %d", len(testData), written)
	}

	// Rate limiter should have been called
	if rateLimiter.getWaitCalled() == 0 {
		t.Error("Rate limiter should have been called")
	}
}

func TestTransfer_Copy_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a reader that will delay to allow cancellation
	src := &mockReader{
		data:      []byte(strings.Repeat("A", 1000)),
		readDelay: 100 * time.Millisecond,
	}
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	written, err := transfer.copy()

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Should have written some data before cancellation
	if written < 0 {
		t.Error("Written bytes should not be negative")
	}
}

func TestTransfer_Copy_ReadError(t *testing.T) {
	ctx := context.Background()
	readErr := errors.New("read error")
	src := &mockReader{err: readErr}
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()

	if err != readErr {
		t.Errorf("Expected read error, got %v", err)
	}

	if written != 0 {
		t.Errorf("Expected 0 bytes written, got %d", written)
	}
}

func TestTransfer_Copy_WriteError(t *testing.T) {
	ctx := context.Background()
	writeErr := errors.New("write error")
	src := strings.NewReader("test data")
	dst := &mockWriter{writeErr: writeErr}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()

	if err != writeErr {
		t.Errorf("Expected write error, got %v", err)
	}

	if written != 0 {
		t.Errorf("Expected 0 bytes written, got %d", written)
	}
}

func TestTransfer_Copy_ShortWrite(t *testing.T) {
	ctx := context.Background()
	src := strings.NewReader("test data")
	dst := &mockWriter{shortWrite: true}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()

	if err != io.ErrShortWrite {
		t.Errorf("Expected io.ErrShortWrite, got %v", err)
	}

	if written == 0 {
		t.Error("Should have written some data before short write error")
	}
}

func TestTransfer_Copy_RateLimiterError(t *testing.T) {
	ctx := context.Background()
	rateLimitErr := errors.New("rate limit error")
	src := strings.NewReader("test data")
	dst := &bytes.Buffer{}
	rateLimiter := &mockRateLimiter{waitError: rateLimitErr}

	transfer := newTransfer(ctx, rateLimiter, dst, src, nil)

	written, err := transfer.copy()

	if err != rateLimitErr {
		t.Errorf("Expected rate limit error, got %v", err)
	}

	// Should have written the data before rate limiter was called
	if written == 0 {
		t.Error("Should have written some data before rate limit error")
	}
}

func TestTransfer_Copy_RateLimiterContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src := strings.NewReader("test data")
	dst := &bytes.Buffer{}
	rateLimiter := &mockRateLimiter{
		shouldWait:   true,
		waitDuration: 200 * time.Millisecond,
	}

	transfer := newTransfer(ctx, rateLimiter, dst, src, nil)

	// Cancel context during rate limiting
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	written, err := transfer.copy()

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Should have written the data before cancellation
	if written == 0 {
		t.Error("Should have written some data before cancellation")
	}
}

func TestTransfer_N_InitiallyZero(t *testing.T) {
	ctx := context.Background()
	src := strings.NewReader("")
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	if transfer.N() != 0 {
		t.Errorf("N() should initially return 0, got %d", transfer.N())
	}
}

func TestTransfer_N_AfterCopy(t *testing.T) {
	ctx := context.Background()
	testData := "Test data for N() method"
	src := strings.NewReader(testData)
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()
	if err != nil {
		t.Fatalf("copy() failed: %v", err)
	}

	if transfer.N() != written {
		t.Errorf("N() should return %d, got %d", written, transfer.N())
	}

	if transfer.N() != int64(len(testData)) {
		t.Errorf("N() should return %d, got %d", len(testData), transfer.N())
	}
}

func TestTransfer_N_Nil(t *testing.T) {
	var transfer *transfer = nil

	if transfer.N() != 0 {
		t.Errorf("N() on nil transfer should return 0, got %d", transfer.N())
	}
}

func TestTransfer_BPS_Nil(t *testing.T) {
	var transfer *transfer = nil

	if transfer.BPS() != 0 {
		t.Errorf("BPS() on nil transfer should return 0, got %f", transfer.BPS())
	}
}

func TestTransfer_BPS_NoGauge(t *testing.T) {
	ctx := context.Background()
	src := strings.NewReader("test")
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	if transfer.BPS() != 0 {
		t.Errorf("BPS() should return 0 when no gauge, got %f", transfer.BPS())
	}
}

func TestTransfer_Copy_EmptyData(t *testing.T) {
	ctx := context.Background()
	src := strings.NewReader("")
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()
	if err != nil {
		t.Errorf("copy() with empty data returned error: %v", err)
	}

	if written != 0 {
		t.Errorf("Expected 0 bytes written for empty data, got %d", written)
	}

	if transfer.N() != 0 {
		t.Errorf("N() should return 0 for empty data, got %d", transfer.N())
	}
}

func TestTransfer_Copy_LargeData(t *testing.T) {
	ctx := context.Background()
	// Create 1MB of test data
	testData := strings.Repeat("ABCDEFGHIJ", 100000) // 1MB
	src := strings.NewReader(testData)
	dst := &bytes.Buffer{}

	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()
	if err != nil {
		t.Errorf("copy() with large data returned error: %v", err)
	}

	expectedSize := int64(len(testData))
	if written != expectedSize {
		t.Errorf("Expected %d bytes written, got %d", expectedSize, written)
	}

	if int64(dst.Len()) != expectedSize {
		t.Errorf("Expected %d bytes in buffer, got %d", expectedSize, dst.Len())
	}

	if transfer.N() != written {
		t.Errorf("N() should return %d, got %d", written, transfer.N())
	}
}

func TestTransfer_Copy_DefaultBuffer(t *testing.T) {
	ctx := context.Background()
	testData := strings.Repeat("X", 50000) // 50KB
	src := strings.NewReader(testData)
	dst := &bytes.Buffer{}

	// Pass nil buffer to test default buffer creation
	transfer := newTransfer(ctx, nil, dst, src, nil)

	written, err := transfer.copy()
	if err != nil {
		t.Errorf("copy() with default buffer returned error: %v", err)
	}

	if written != int64(len(testData)) {
		t.Errorf("Expected %d bytes written, got %d", len(testData), written)
	}

	// Verify default buffer was created (32KB)
	if len(transfer.b) != 32*1024 {
		t.Errorf("Expected default buffer size 32768, got %d", len(transfer.b))
	}
}

// Benchmark tests
func BenchmarkTransfer_Copy_Small(b *testing.B) {
	testData := strings.Repeat("A", 1024) // 1KB
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := strings.NewReader(testData)
		dst := &bytes.Buffer{}
		transfer := newTransfer(ctx, nil, dst, src, nil)
		_, _ = transfer.copy()
	}
}

func BenchmarkTransfer_Copy_Large(b *testing.B) {
	testData := strings.Repeat("A", 1024*1024) // 1MB
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := strings.NewReader(testData)
		dst := &bytes.Buffer{}
		transfer := newTransfer(ctx, nil, dst, src, nil)
		_, _ = transfer.copy()
	}
}

func BenchmarkTransfer_Copy_WithRateLimit(b *testing.B) {
	testData := strings.Repeat("A", 10240) // 10KB
	ctx := context.Background()
	rateLimiter := &mockRateLimiter{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := strings.NewReader(testData)
		dst := &bytes.Buffer{}
		transfer := newTransfer(ctx, rateLimiter, dst, src, nil)
		_, _ = transfer.copy()
	}
}

func BenchmarkTransfer_N(b *testing.B) {
	ctx := context.Background()
	src := strings.NewReader("test data")
	dst := &bytes.Buffer{}
	transfer := newTransfer(ctx, nil, dst, src, nil)

	// Copy some data first
	_, _ = transfer.copy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transfer.N()
	}
}
