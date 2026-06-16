# grab

[![Nightly Tests](https://github.com/sebrandon1/grab/actions/workflows/nightly-grab.yml/badge.svg)](https://github.com/sebrandon1/grab/actions/workflows/nightly-grab.yml)
[![Pre-main Tests](https://github.com/sebrandon1/grab/actions/workflows/pre-main.yml/badge.svg)](https://github.com/sebrandon1/grab/actions/workflows/pre-main.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/sebrandon1/grab)](https://github.com/sebrandon1/grab/blob/main/go.mod)
[![License](https://img.shields.io/github/license/sebrandon1/grab)](https://github.com/sebrandon1/grab/blob/main/LICENSE)

A minimal Go CLI and library for downloading files from the internet, inspired by cURL and wget.

## Features

- Download files from URLs with automatic filename detection
- Concurrent multi-file downloads
- Real-time progress tracking with verbose mode
- Built-in file hash computation (MD5, SHA1, SHA256)
- Usable as a Go library or standalone binary

## Quick Start

```bash
# Install (macOS Apple Silicon — see docs/installation.md for all platforms)
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-darwin-arm64 -o grab
chmod +x grab

# Download a file
grab download https://example.com/file.tar.gz

# Download multiple files concurrently with progress
grab download -v https://example.com/a.tar.gz https://example.com/b.tar.gz

# Verify file integrity
grab hash file.tar.gz --type sha256
```

## Library Usage

```go
import "github.com/sebrandon1/grab/lib"

ch, err := lib.DownloadBatch(context.Background(), urls)
for resp := range ch {
    log.Printf("%s: %v", resp.Filename, resp.Err)
}
```

See [lib/README.md](lib/README.md) for the full API reference.

## Guides

| Document | Description |
|---|---|
| [Installation](docs/installation.md) | Pre-built binaries for all platforms, building from source |
| [CLI Reference](docs/cli-reference.md) | Commands, flags, and usage examples |
| [Library API](lib/README.md) | Go library types, functions, and examples |
| [Contributing](CONTRIBUTING.md) | Code style, testing, and PR guidelines |
| [Security](SECURITY.md) | Vulnerability reporting policy |

## Development

```bash
make build          # Build the binary
make test           # Run tests with coverage
make lint           # Run golangci-lint
make vet            # Run go vet
```

## License

MIT
