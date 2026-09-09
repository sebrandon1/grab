# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A minimal Go CLI and library for downloading files from the internet, inspired by cURL and wget. Supports concurrent downloads, progress tracking, and hash computation.

## Common Commands

### Build
```bash
make build
```

### Run
```bash
# Download a file
./grab download https://example.com/file.zip

# Download multiple files
./grab download https://example.com/file1.zip https://example.com/file2.zip

# Verbose mode with progress
./grab download -v https://example.com/file.zip

# Compute file hash
./grab hash file.zip --type sha256
```

### Test
```bash
go test ./...
make test
```

### Lint and Vet
```bash
make vet
make lint
```

## Architecture

- **`cmd/`** - CLI command implementations using Cobra
- **`lib/`** - Core download library with hash computation
- **`main.go`** - Application entry point

## Features

- Download files from URLs to local directory
- Automatic filename detection from Content-Disposition headers
- Concurrent multi-file downloads
- Real-time progress tracking (verbose mode)
- Built-in hash computation (MD5, SHA1, SHA256, SHA512)
- Usable as Go library or standalone CLI

## Library Usage

```go
import (
    "log"
    "github.com/sebrandon1/grab/lib"
)

client := lib.NewClient()
req, err := lib.NewRequest(".", "https://example.com/file.zip")
if err != nil {
    log.Fatal(err)
}
resp := client.Do(req)
if err := resp.Err(); err != nil {
    log.Fatal(err)
}
```

## Requirements

- Go 1.25+

## Code Style

- Follow standard Go conventions
- Use `go fmt` before committing
- Run `golangci-lint` for linting
