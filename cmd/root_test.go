package cmd

import (
	"bytes"
	"testing"
)

// TestRootCmd_VersionOutput verifies the --version flag prints the version,
// commit, and build date injected via SetVersionInfo.
func TestRootCmd_VersionOutput(t *testing.T) {
	var buf bytes.Buffer
	origOut := rootCmd.OutOrStdout()
	origVersion := rootCmd.Version
	t.Cleanup(func() {
		rootCmd.SetOut(origOut)
		rootCmd.Version = origVersion
		rootCmd.SetArgs(nil)
	})

	SetVersionInfo("v1.2.3", "deadbee", "2026-10-14T00:00:00Z")
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}

	want := "grab version v1.2.3 (commit deadbee, built 2026-10-14T00:00:00Z)\n"
	if got := buf.String(); got != want {
		t.Errorf("version output\ngot:  %q\nwant: %q", got, want)
	}
}
