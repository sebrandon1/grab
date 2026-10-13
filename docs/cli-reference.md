# CLI Reference

## Commands

| Command | Description |
|---------|-------------|
| `grab download [url]...` | Download one or more files concurrently |
| `grab hash [file]` | Compute the hash of a local file |

## Download

Download one or more files from URLs. Multiple URLs are fetched concurrently.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--verbose` | `-v` | false | Show real-time progress bar (single URL) or start/completion messages (multiple URLs) |
| `--output` | `-o` | — | Output file path (single URL) or destination directory (multiple URLs) |
| `--retries` | — | `0` | Number of retry attempts for transient HTTP errors (5xx, 429) |
| `--timeout` | — | `0s` | Network phase timeout (dial, TLS handshake, response headers), e.g. `30s`. Does not limit total transfer time. `0` disables |
| `--sha256` | — | — | Verify the download against a hex-encoded SHA-256 checksum; on mismatch the file is deleted and the command exits 1 (single URL only) |
| `--limit-rate` | — | — | Limit download rate, e.g. `200k` or `2m`; caps aggregate throughput across concurrent downloads |
| `--file` | `-f` | — | Read URLs from a file, one per line (blank lines and `#` comments ignored); appended to positional URLs; duplicate URLs are downloaded once |

### Single file

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
```

### Multiple files (concurrent)

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip \
             https://github.com/sebrandon1/grab/archive/refs/tags/v1.0.0.zip
```

### Download URLs from a file

```bash
# urls.txt: one URL per line; blank lines and # comments are ignored
grab download -f urls.txt

# Combine with positional URLs (all downloaded concurrently)
grab download https://example.com/extra.zip -f urls.txt
```

### Verbose progress output

```bash
grab download -v https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
```

Example output:

```
Downloading: [======================================= ]  99.34% (135728945/136421772 bytes)
Downloaded: main.zip (size: 136421772 bytes)
```

### Checksum-verified download

```bash
grab download --sha256 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
             https://example.com/file.tar.gz
```

Downloads the file and verifies its SHA-256. On mismatch the downloaded file
is deleted and the command exits 1. Requires exactly one URL; can be combined
with `--output`.

### Rate-limited download

```bash
grab download --limit-rate 200k https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
```

Caps download throughput at 200 KB/s. The limit is aggregate: with multiple
URLs, all concurrent transfers share it. Accepts plain bytes (`1048576`),
`k`/`K` (×1024), or `m`/`M` (×1024²) suffixes, inspired by wget's
`--limit-rate` (integers only).

## Hash

Compute file hashes to verify integrity.

### Flags

| Flag | Short | Default | Accepted values |
|------|-------|---------|-----------------|
| `--type` | `-t` | `sha256` | `sha256`, `sha512`, `sha1`, `md5` |

```bash
# SHA-256 (default)
grab hash myfile.tar.gz

# Explicit SHA-256
grab hash myfile.tar.gz --type sha256

# SHA-512
grab hash myfile.tar.gz -t sha512

# MD5
grab hash myfile.tar.gz -t md5

# SHA-1
grab hash myfile.tar.gz -t sha1
```

### Download and verify workflow

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
grab hash main.zip
```

## Help

```bash
grab --help
grab download --help
grab hash --help
```
