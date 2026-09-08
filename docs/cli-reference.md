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
| `--verbose` | `-v` | false | Show real-time progress bar and download summary |

### Single file

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
```

### Multiple files (concurrent)

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip \
             https://github.com/sebrandon1/grab/archive/refs/tags/v1.0.0.zip
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

## Hash

Compute file hashes to verify integrity.

### Flags

| Flag | Short | Default | Accepted values |
|------|-------|---------|-----------------|
| `--type` | `-t` | `sha256` | `sha256`, `sha1`, `md5` |

```bash
# SHA-256 (default)
grab hash myfile.tar.gz

# Explicit SHA-256
grab hash myfile.tar.gz --type sha256

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
