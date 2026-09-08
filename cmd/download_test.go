package cmd

import (
	"context"
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
