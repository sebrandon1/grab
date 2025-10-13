# grab

[![Nightly Tests](https://github.com/sebrandon1/grab/actions/workflows/nightly-grab.yml/badge.svg)](https://github.com/sebrandon1/grab/actions/workflows/nightly-grab.yml)
[![Pre-main Tests](https://github.com/sebrandon1/grab/actions/workflows/pre-main.yml/badge.svg)](https://github.com/sebrandon1/grab/actions/workflows/pre-main.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/sebrandon1/grab)](https://github.com/sebrandon1/grab/blob/main/go.mod)
[![License](https://img.shields.io/github/license/sebrandon1/grab)](https://github.com/sebrandon1/grab/blob/main/LICENSE)

A minimal Go CLI and library for downloading files from the internet, inspired by cURL and wget.

## Features

- Download files from URLs to your local directory
- Automatic filename detection from Content-Disposition headers or URL paths
- Download multiple files concurrently
- Real-time progress tracking with verbose mode
- Built-in file hash computation (MD5, SHA1, SHA256)
- Simple, modern CLI with `grab download [url]...`
- Usable as a Go library or standalone binary

## Install

### Option 1: Download Pre-built Binaries (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/sebrandon1/grab/releases):

**Linux (x86_64):**
```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-linux-amd64 -o grab
chmod +x grab
```

**macOS (Intel):**
```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-darwin-amd64 -o grab
chmod +x grab
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-darwin-arm64 -o grab
chmod +x grab
```

**Windows:**
```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-windows-amd64.exe -o grab.exe
```

### Option 2: Build from Source

Requirements: Go 1.25 or newer

```bash
git clone https://github.com/sebrandon1/grab.git
cd grab
make build
```

This will produce a `grab` binary in the repo root.

## Usage

### CLI

**Download a single file:**
```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
```

**Download multiple files concurrently:**
```bash
grab download https://go.dev/dl/go1.21.5.src.tar.gz https://go.dev/dl/go1.21.4.src.tar.gz
```

**Download with verbose progress output:**
```bash
grab download https://go.dev/dl/go1.21.5.darwin-amd64.tar.gz --verbose
```
Example output:
```
Downloading: [======================================= ]  99.34% (135728945/136421772 bytes)
Downloaded: go1.21.5.darwin-amd64.tar.gz (size: 136421772 bytes)
```

**Download from GitHub releases:**
```bash
grab download https://github.com/golang/go/archive/refs/tags/go1.21.5.tar.gz
```

**Compute file hashes:**
```bash
# Download and verify file integrity
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
grab hash main.zip --type sha256

# Different hash algorithms
grab hash myfile.tar.gz --type md5
grab hash myfile.tar.gz --type sha1
```

**Get help:**
```bash
grab --help
grab download --help
grab hash --help
```

### As a Go Library

See [lib/README.md](lib/README.md) for full documentation of the public API.

```go
package main

import (
	"log"
	"github.com/sebrandon1/grab/lib"
	"context"
)

func main() {
	urls := []string{
		"https://github.com/sebrandon1/grab/archive/refs/heads/main.zip",
		"https://go.dev/dl/go1.21.5.src.tar.gz",
	}
	ch, err := lib.DownloadBatch(context.Background(), urls)
	if err != nil {
		log.Fatal(err)
	}
	for resp := range ch {
		if resp.Err != nil {
			log.Printf("Failed: %s (%v)", resp.Filename, resp.Err)
		} else {
			log.Printf("Downloaded: %s", resp.Filename)
		}
	}
}
```

## License

MIT
