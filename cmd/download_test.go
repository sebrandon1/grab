package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

func newTestCmdWithOutput(outputPath string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("output", "o", outputPath, "")
	return cmd
}

func newTestCmdWithLimitRate(value string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("limit-rate", value, "")
	return cmd
}

func newTestCmdWithFile(path string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("file", "f", path, "")
	return cmd
}

// chdirTemp switches the process working directory to a fresh temp dir for the
// duration of t and restores it on cleanup.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

func serveFile(t *testing.T, content, filename string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		_, _ = w.Write([]byte(content))
	}))
}

func serve404(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
}

// serveDelayed serves a 2 KiB body in two 1 KiB chunks under filename: the
// first chunk is flushed immediately, the second after a delay longer than
// one progress-ticker interval, so verbose progress renders mid-download. A
// non-positive contentLength omits Content-Length (unknown size). HEAD
// filename probes get the headers without the delay.
func serveDelayed(t *testing.T, filename string, contentLength int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		if contentLength > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
		}
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
}

func TestRunDownload_SingleFile(t *testing.T) {
	srv := serveFile(t, "hello world", "testfile.bin")
	defer srv.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmd(), []string{srv.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 downloaded file, got %d", len(entries))
	}
}

func TestRunDownload_MultiFile(t *testing.T) {
	srv1 := serveFile(t, "file one", "file1.bin")
	defer srv1.Close()
	srv2 := serveFile(t, "file two", "file2.bin")
	defer srv2.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmd(), []string{srv1.URL, srv2.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 downloaded files, got %d", len(entries))
	}
}

func TestRunDownload_FailedURL(t *testing.T) {
	srv := serve404(t)
	defer srv.Close()
	chdirTemp(t)

	code := runDownload(newTestCmd(), []string{srv.URL + "/missing.bin"}, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code for 404 response")
	}
}

func TestRunDownload_InvalidURL(t *testing.T) {
	chdirTemp(t)

	code := runDownload(newTestCmd(), []string{"://not-a-url"}, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code for invalid URL")
	}
}

func TestRunDownload_PartialFailure(t *testing.T) {
	good := serveFile(t, "good content", "good.bin")
	defer good.Close()
	bad := serve404(t)
	defer bad.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmd(), []string{good.URL, bad.URL + "/missing.bin"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 (one failure), got %d", code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bin") {
			found = true
		}
	}
	if !found {
		t.Error("expected the successful download to be present on disk")
	}
}

func TestRunDownload_VerboseSingleFile(t *testing.T) {
	srv := serveFile(t, strings.Repeat("x", 1024), "testfile.bin")
	defer srv.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmd(), []string{srv.URL}, true)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "testfile.bin")); err != nil {
		t.Fatalf("expected testfile.bin to exist: %v", err)
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything it printed. Stdout is restored via defer so a panic inside fn
// cannot leave it hijacked for the rest of the package run.
func captureStdout(t *testing.T, fn func() int) (output string, exitCode int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	exitCode = fn()

	_ = w.Close()
	b, _ := io.ReadAll(r)
	return string(b), exitCode
}

// runDownloadCapture calls runDownload and captures its stdout output.
func runDownloadCapture(t *testing.T, cmd *cobra.Command, args []string, verbose bool) (output string, exitCode int) {
	t.Helper()
	return captureStdout(t, func() int {
		return runDownload(cmd, args, verbose)
	})
}

// A verbose single-file download with an explicit -o path must render the
// same live progress bar as the default path.
func TestRunDownload_VerboseOutputFile(t *testing.T) {
	srv := serveDelayed(t, "progress.bin", 2048)
	defer srv.Close()
	outPath := filepath.Join(t.TempDir(), "progress.bin")

	out, code := runDownloadCapture(t, newTestCmdWithOutput(outPath), []string{srv.URL}, true)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Downloading: [") {
		t.Errorf("expected a progress bar line in verbose output, got %q", out)
	}
	if !strings.Contains(out, "50.00%") {
		t.Errorf("expected the progress bar to show mid-transfer progress, got %q", out)
	}
}

// With no Content-Length the bar cannot show a percentage; verbose output
// still renders byte progress instead.
func TestRunDownload_VerboseUnknownSize(t *testing.T) {
	srv := serveDelayed(t, "nosize.bin", 0)
	defer srv.Close()
	chdirTemp(t)

	out, code := runDownloadCapture(t, newTestCmd(), []string{srv.URL}, true)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Downloading: 1024 bytes complete") {
		t.Errorf("expected byte-count progress for unknown-size download, got %q", out)
	}
}

func TestRunDownload_VerboseMultiFile(t *testing.T) {
	srv1 := serveFile(t, "alpha", "alpha.bin")
	defer srv1.Close()
	srv2 := serveFile(t, "beta", "beta.bin")
	defer srv2.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmd(), []string{srv1.URL, srv2.URL}, true)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 files, got %d", len(entries))
	}
}

func TestRunDownload_OutputFile(t *testing.T) {
	srv := serveFile(t, "hello output", "ignored.bin")
	defer srv.Close()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "custom.bin")

	code := runDownload(newTestCmdWithOutput(outPath), []string{srv.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected output file at %s: %v", outPath, err)
	}
	if string(data) != "hello output" {
		t.Errorf("unexpected content: %q", data)
	}
}

func TestRunDownload_OutputFileInvalidURL(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "custom.bin")

	code := runDownload(newTestCmdWithOutput(outPath), []string{"://not-a-url"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for invalid URL with --output, got %d", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("expected no output file, stat err: %v", err)
	}
}

func TestRunDownload_OutputFileFailedURL(t *testing.T) {
	srv := serve404(t)
	defer srv.Close()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "custom.bin")

	code := runDownload(newTestCmdWithOutput(outPath), []string{srv.URL + "/missing.bin"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for failed download with --output, got %d", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("expected no output file after failure, stat err: %v", err)
	}
}

func TestRunDownload_OutputDirectory(t *testing.T) {
	srv := serveFile(t, "hello dir", "dirfile.bin")
	defer srv.Close()
	dir := t.TempDir()

	code := runDownload(newTestCmdWithOutput(dir), []string{srv.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "dirfile.bin")); err != nil {
		t.Errorf("expected dirfile.bin in output dir: %v", err)
	}
}

func newTestCmdWithSHA256(sumHex string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("sha256", sumHex, "")
	return cmd
}

func TestRunDownload_ChecksumSuccess(t *testing.T) {
	content := "hello checksum"
	srv := serveFile(t, content, "checksum-ok.bin")
	defer srv.Close()
	dir := chdirTemp(t)

	sum := sha256.Sum256([]byte(content))

	out, code := runDownloadCapture(t, newTestCmdWithSHA256(hex.EncodeToString(sum[:])), []string{srv.URL}, true)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "checksum-ok.bin")); err != nil {
		t.Errorf("expected checksum-ok.bin on disk: %v", err)
	}
	if !strings.Contains(out, "sha256 verified") {
		t.Errorf("expected verbose summary to note the verified checksum, got %q", out)
	}
}

func TestRunDownload_ChecksumMismatch(t *testing.T) {
	content := "hello checksum"
	srv := serveFile(t, content, "checksum-bad.bin")
	defer srv.Close()
	dir := chdirTemp(t)

	badSum := sha256.Sum256([]byte("different content"))

	code := runDownload(newTestCmdWithSHA256(hex.EncodeToString(badSum[:])), []string{srv.URL}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 on checksum mismatch, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "checksum-bad.bin")); !os.IsNotExist(err) {
		t.Errorf("expected checksum-bad.bin to be deleted on mismatch, stat err: %v", err)
	}
}

func TestRunDownload_ChecksumInvalidValue(t *testing.T) {
	cases := []struct {
		name string
		sum  string
	}{
		{"too short", "abcd"},
		{"not hex", strings.Repeat("z", 2*sha256.Size)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chdirTemp(t)

			// The URL is never contacted: validation must fail before any request.
			code := runDownload(newTestCmdWithSHA256(tc.sum), []string{"http://example.com/file.bin"}, false)
			if code != 1 {
				t.Errorf("expected exit 1 for %q, got %d", tc.sum, code)
			}
		})
	}
}

func TestParseByteRate(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"100", 100, false},
		{"1k", 1024, false},
		{"1K", 1024, false},
		{"500k", 500 * 1024, false},
		{"2m", 2 * 1024 * 1024, false},
		{"2M", 2 * 1024 * 1024, false},
		{"", 0, true},
		{"abc", 0, true},
		{"10x", 0, true},
		{"-5", 0, true},
		{"0", 0, true},
		{"99999999999999999999", 0, true},
		{"17592186044417m", 0, true},
		{"9223372036854775807m", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseByteRate(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseByteRate(%q) = %d, expected error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseByteRate(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseByteRate(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestRunDownload_ChecksumMultipleURLs(t *testing.T) {
	chdirTemp(t)
	sum := sha256.Sum256([]byte("x"))

	code := runDownload(newTestCmdWithSHA256(hex.EncodeToString(sum[:])), []string{"http://a.example/f.bin", "http://b.example/f.bin"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for --sha256 with multiple URLs, got %d", code)
	}
}

func TestRunDownload_ChecksumInvalidURL(t *testing.T) {
	chdirTemp(t)
	sum := sha256.Sum256([]byte("x"))

	code := runDownload(newTestCmdWithSHA256(hex.EncodeToString(sum[:])), []string{"://not-a-url"}, false)
	if code != 1 {
		t.Fatal("expected non-zero exit code for invalid URL with --sha256")
	}
}

func TestRunDownload_ChecksumWithOutput(t *testing.T) {
	content := "hello checksum output"
	srv := serveFile(t, content, "ignored.bin")
	defer srv.Close()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "custom-checksum.bin")

	sum := sha256.Sum256([]byte(content))

	cmd := newTestCmdWithOutput(outPath)
	cmd.Flags().String("sha256", hex.EncodeToString(sum[:]), "")
	code := runDownload(cmd, []string{srv.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected output file at %s: %v", outPath, err)
	}
	if string(data) != content {
		t.Errorf("unexpected content: %q", data)
	}
}

func TestRunDownload_LimitRateInvalid(t *testing.T) {
	chdirTemp(t)

	code := runDownload(newTestCmdWithLimitRate("not-a-rate"), []string{"http://example.com/file.bin"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for invalid --limit-rate, got %d", code)
	}
}

func TestRunDownload_LimitRateInvalidURL(t *testing.T) {
	chdirTemp(t)

	code := runDownload(newTestCmdWithLimitRate("100k"), []string{"://not-a-url"}, false)
	if code != 1 {
		t.Fatal("expected non-zero exit code for invalid URL with --limit-rate")
	}
}

// The shared limiter paces transfers after its 64KB burst is consumed, so a
// 256KB payload at 128KB/s takes ~1.5s. Only a loose lower bound is asserted
// to stay robust on loaded CI runners.
func TestRunDownload_LimitRate(t *testing.T) {
	srv := serveFile(t, strings.Repeat("x", 256*1024), "limited.bin")
	defer srv.Close()
	chdirTemp(t)

	start := time.Now()
	code := runDownload(newTestCmdWithLimitRate("128k"), []string{srv.URL}, false)
	elapsed := time.Since(start)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if elapsed < time.Second {
		t.Errorf("expected rate-limited download to take >= 1s, took %v", elapsed)
	}
}

func TestRunDownload_LimitRateWithOutput(t *testing.T) {
	srv := serveFile(t, strings.Repeat("x", 128*1024), "ignored.bin")
	defer srv.Close()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "limited.bin")

	start := time.Now()
	cmd := newTestCmdWithOutput(outPath)
	cmd.Flags().String("limit-rate", "64k", "")
	code := runDownload(cmd, []string{srv.URL}, false)
	elapsed := time.Since(start)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("expected rate-limited download to take >= 500ms, took %v", elapsed)
	}
}

func TestRunDownload_OutputDirectoryMissing(t *testing.T) {
	srv := serveFile(t, "a", "a.bin")
	defer srv.Close()
	dir := t.TempDir()

	// Two distinct URLs: the batch path (and its directory check) is only
	// reached with multiple URLs, since duplicates are collapsed to one.
	code := runDownload(newTestCmdWithOutput(filepath.Join(dir, "missing")), []string{srv.URL, srv.URL + "/second"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing output directory, got %d", code)
	}
}

func TestRunDownload_OutputNotDirectory(t *testing.T) {
	srv := serveFile(t, "a", "a.bin")
	defer srv.Close()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "plain-file")

	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	code := runDownload(newTestCmdWithOutput(filePath), []string{srv.URL, srv.URL + "/second"}, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for output path that is not a directory, got %d", code)
	}
}

// Two concurrent 96KB transfers share one 64k limiter: (192KB - 64KB burst) /
// 64KB/s = 2s. The lower bound also fails if the limiter were accidentally
// per-request, which would finish in ~0.5s.
func TestRunDownload_LimitRateMultipleURLs(t *testing.T) {
	srv1 := serveFile(t, strings.Repeat("a", 96*1024), "multi1.bin")
	defer srv1.Close()
	srv2 := serveFile(t, strings.Repeat("b", 96*1024), "multi2.bin")
	defer srv2.Close()
	dir := chdirTemp(t)

	start := time.Now()
	code := runDownload(newTestCmdWithLimitRate("64k"), []string{srv1.URL, srv2.URL}, false)
	elapsed := time.Since(start)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if elapsed < 1500*time.Millisecond {
		t.Errorf("expected aggregate-limited downloads to take >= 1.5s, took %v", elapsed)
	}
	for _, name := range []string{"multi1.bin", "multi2.bin"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s on disk: %v", name, err)
		}
	}
}

// writeURLFile writes lines to a new file in a temp dir and returns its path.
func writeURLFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "urls.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunDownload_URLFile(t *testing.T) {
	srv1 := serveFile(t, "file one", "from-file1.bin")
	defer srv1.Close()
	srv2 := serveFile(t, "file two", "from-file2.bin")
	defer srv2.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmdWithFile(writeURLFile(t,
		"# two real URLs, one comment, blank lines around them",
		"",
		srv1.URL+"\r", // CRLF line ending must be tolerated
		"   ",
		srv2.URL,
		"# trailing comment",
	)), nil, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, name := range []string{"from-file1.bin", "from-file2.bin"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s on disk: %v", name, err)
		}
	}
}

func TestRunDownload_URLFileWithArgs(t *testing.T) {
	srv1 := serveFile(t, "positional", "positional.bin")
	defer srv1.Close()
	srv2 := serveFile(t, "from file", "from-file.bin")
	defer srv2.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmdWithFile(writeURLFile(t, "", srv2.URL, "# comment")), []string{srv1.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 downloaded files (positional + file URL), got %d", len(entries))
	}
}

// A URL given both positionally and in the file must be fetched once:
// duplicate requests would race to write the same output file.
func TestRunDownload_URLFileDuplicateURLs(t *testing.T) {
	// lib probes each URL with a HEAD before the transfer, so count GETs.
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets.Add(1)
		}
		w.Header().Set("Content-Disposition", `attachment; filename="dup.bin"`)
		_, _ = w.Write([]byte("dup"))
	}))
	defer srv.Close()
	dir := chdirTemp(t)

	code := runDownload(newTestCmdWithFile(writeURLFile(t, srv.URL)), []string{srv.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if n := gets.Load(); n != 1 {
		t.Errorf("expected exactly 1 GET for the duplicate URL, got %d", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "dup.bin")); err != nil {
		t.Errorf("expected dup.bin on disk: %v", err)
	}
}

func TestRunDownload_URLFileMissing(t *testing.T) {
	chdirTemp(t)

	code := runDownload(newTestCmdWithFile(filepath.Join(t.TempDir(), "missing.txt")), nil, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing URL file, got %d", code)
	}
}

func TestRunDownload_URLFileEmpty(t *testing.T) {
	srv := serveFile(t, "x", "unused.bin")
	defer srv.Close()
	chdirTemp(t)

	code := runDownload(newTestCmdWithFile(writeURLFile(t, "# only comments", "", "   ")), nil, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for URL file with no URLs, got %d", code)
	}
}

func TestRunDownload_URLFileLongLine(t *testing.T) {
	chdirTemp(t)
	// A line over bufio.Scanner's 64KB token limit must surface as a read
	// error, not a silently truncated URL list.

	code := runDownload(newTestCmdWithFile(writeURLFile(t, strings.Repeat("a", 128*1024))), nil, false)
	if code != 1 {
		t.Fatalf("expected exit 1 for URL file with oversized line, got %d", code)
	}
}

func TestDownloadCmdArgs(t *testing.T) {
	if err := downloadCmd.Args(downloadCmd, nil); err == nil {
		t.Error("expected error for no positional URLs without --file")
	}

	// pflag only marks a flag Changed on an explicit Set or CLI parse —
	// registration alone does not — and the Args validator keys on Changed.
	withFile := newTestCmdWithFile("")
	if err := withFile.Flags().Set("file", "urls.txt"); err != nil {
		t.Fatal(err)
	}
	if err := downloadCmd.Args(withFile, nil); err != nil {
		t.Errorf("expected no error with --file set: %v", err)
	}
	if err := downloadCmd.Args(downloadCmd, []string{"http://example.com"}); err != nil {
		t.Errorf("expected no error with positional URL: %v", err)
	}
}

// The --retries and --retry-delay flags must reach lib.DefaultClient before
// any transfer starts: a 429 is retried after the configured delay.
func TestRunDownload_RetriesWithRetryDelay(t *testing.T) {
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && gets.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="retry.bin"`)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	dir := chdirTemp(t)

	cmd := newTestCmd()
	cmd.Flags().Int("retries", 1, "")
	cmd.Flags().Duration("retry-delay", 50*time.Millisecond, "")
	// runDownload configures the shared lib.DefaultClient; restore it so the
	// retry settings do not leak into later tests.
	origLimit, origDelay := lib.DefaultClient.RetryLimit, lib.DefaultClient.RetryDelay
	t.Cleanup(func() {
		lib.DefaultClient.RetryLimit, lib.DefaultClient.RetryDelay = origLimit, origDelay
	})

	start := time.Now()
	code := runDownload(cmd, []string{srv.URL}, false)
	elapsed := time.Since(start)
	if code != 0 {
		t.Fatalf("expected exit 0 after retrying the 429, got %d", code)
	}
	if n := gets.Load(); n != 2 {
		t.Errorf("expected exactly 2 GETs (429 + retry), got %d", n)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected the retry to wait out the 50ms --retry-delay, retried after %v", elapsed)
	}
	if _, err := os.Stat(filepath.Join(dir, "retry.bin")); err != nil {
		t.Errorf("expected retry.bin on disk: %v", err)
	}
}
