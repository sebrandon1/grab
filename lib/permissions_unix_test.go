//go:build unix

package lib

import (
	"syscall"
	"testing"
)

// TestClient_DownloadedFilePermissions_UmaskZero proves the download code
// requests 0644 rather than 0666: with a permissive umask the old mode would
// leave the file group- and world-writable.
func TestClient_DownloadedFilePermissions_UmaskZero(t *testing.T) {
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })

	if perm := downloadForPermTest(t).Mode().Perm(); perm != 0o644 {
		t.Errorf("downloaded file mode = %#o with umask 0, want 0644", perm)
	}
}
