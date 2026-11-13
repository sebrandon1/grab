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

Requirements: Go 1.26 or newer

`go.mod` declares `go 1.26.0` as the minimum and suggests a patched compiler with
`toolchain go1.26.11`. With the default `GOTOOLCHAIN=auto`, Go selects that toolchain
automatically. If you build with `GOTOOLCHAIN=local`, any Go 1.26.x is accepted, but you
are responsible for using a release that includes security fixes such as GO-2026-6809
(Go 1.26.11 or newer).

```bash
git clone https://github.com/sebrandon1/grab.git
cd grab
make build
```

This will produce a `grab` binary in the repo root.
