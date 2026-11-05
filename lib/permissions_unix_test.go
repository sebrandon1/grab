//go:build unix

package lib

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestMkdirp_UmaskZero proves mkdirp requests 0755 rather than 0777: with a
// permissive umask the old mode would leave created directories
// group- and world-writable.
func TestMkdirp_UmaskZero(t *testing.T) {
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })

	dir := t.TempDir()
	if err := mkdirp(filepath.Join(dir, "a", "b", "file")); err != nil {
		t.Fatalf("mkdirp() returned error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "a"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("created directory mode = %#o with umask 0, want 0755", perm)
	}
}
