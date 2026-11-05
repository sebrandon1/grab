package cmd

import (
	"bytes"
	"strings"
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

// TestRootCmd_HelpListsAllHashAlgorithms guards the root help text against
// drifting from the hash command: every algorithm the hash command accepts
// must be advertised.
func TestRootCmd_HelpListsAllHashAlgorithms(t *testing.T) {
	var buf bytes.Buffer
	origOut := rootCmd.OutOrStdout()
	t.Cleanup(func() {
		rootCmd.SetOut(origOut)
		rootCmd.SetArgs(nil)
	})

	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}

	help := buf.String()
	for _, alg := range []string{"MD5", "SHA1", "SHA256", "SHA512"} {
		if !strings.Contains(help, alg) {
			t.Errorf("root help does not mention %s\n%s", alg, help)
		}
	}
}
