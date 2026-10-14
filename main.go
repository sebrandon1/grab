package main

import (
	"github.com/sebrandon1/grab/cmd"
)

// Injected at build time via -ldflags (see Makefile and
// .github/workflows/release-binaries.yaml); the defaults cover a plain
// `go build`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
