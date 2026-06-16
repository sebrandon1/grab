# Installation

## Pre-built Binaries (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/sebrandon1/grab/releases).

### Linux (x86_64)

```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-linux-amd64 -o grab
chmod +x grab
```

### macOS (Intel)

```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-darwin-amd64 -o grab
chmod +x grab
```

### macOS (Apple Silicon)

```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-darwin-arm64 -o grab
chmod +x grab
```

### Windows

```bash
curl -L https://github.com/sebrandon1/grab/releases/latest/download/grab-windows-amd64.exe -o grab.exe
```

## Build from Source

Requirements: Go 1.24 or newer

```bash
git clone https://github.com/sebrandon1/grab.git
cd grab
make build
```

This will produce a `grab` binary in the repo root.
