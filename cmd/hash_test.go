package cmd

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const hashTestContent = "hello grab"

// newHashCmd creates a fresh cobra.Command with the "type" flag registered.
func newHashCmd(hashType string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("type", "t", hashType, "")
	return cmd
}

// runHashCapture calls runHash and captures its stdout output.
func runHashCapture(t *testing.T, cmd *cobra.Command, path string) (output string, exitCode int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w

	exitCode = runHash(cmd, []string{path})

	_ = w.Close()
	os.Stdout = orig

	b, _ := io.ReadAll(r)
	return strings.TrimRight(string(b), "\n"), exitCode
}

func writeHashTestFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "hashtest-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(hashTestContent); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

func TestHashCmd_SHA256(t *testing.T) {
	path := writeHashTestFile(t)
	out, code := runHashCapture(t, newHashCmd("sha256"), path)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	h := sha256.Sum256([]byte(hashTestContent))
	want := hex.EncodeToString(h[:]) + "  " + path
	if out != want {
		t.Errorf("sha256\ngot:  %s\nwant: %s", out, want)
	}
}

func TestHashCmd_SHA1(t *testing.T) {
	path := writeHashTestFile(t)
	out, code := runHashCapture(t, newHashCmd("sha1"), path)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	h := sha1.Sum([]byte(hashTestContent))
	want := hex.EncodeToString(h[:]) + "  " + path
	if out != want {
		t.Errorf("sha1\ngot:  %s\nwant: %s", out, want)
	}
}

func TestHashCmd_MD5(t *testing.T) {
	path := writeHashTestFile(t)
	out, code := runHashCapture(t, newHashCmd("md5"), path)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	h := md5.Sum([]byte(hashTestContent))
	want := hex.EncodeToString(h[:]) + "  " + path
	if out != want {
		t.Errorf("md5\ngot:  %s\nwant: %s", out, want)
	}
}

func TestHashCmd_SHA512(t *testing.T) {
	path := writeHashTestFile(t)
	out, code := runHashCapture(t, newHashCmd("sha512"), path)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	h := sha512.Sum512([]byte(hashTestContent))
	want := hex.EncodeToString(h[:]) + "  " + path
	if out != want {
		t.Errorf("sha512\ngot:  %s\nwant: %s", out, want)
	}
}

func TestHashCmd_MissingFile(t *testing.T) {
	_, code := runHashCapture(t, newHashCmd("sha256"), "/nonexistent/file.bin")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
}

func TestHashCmd_InvalidType(t *testing.T) {
	path := writeHashTestFile(t)
	_, code := runHashCapture(t, newHashCmd("blake2"), path)
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown hash type")
	}
}
