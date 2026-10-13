package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	code := runDownload(newTestCmdWithSHA256(hex.EncodeToString(sum[:])), []string{srv.URL}, false)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "checksum-ok.bin")); err != nil {
		t.Errorf("expected checksum-ok.bin on disk: %v", err)
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
