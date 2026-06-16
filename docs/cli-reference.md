# CLI Reference

## Download

Download one or more files from URLs.

### Single file

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
```

### Multiple files (concurrent)

```bash
grab download https://go.dev/dl/go1.21.5.src.tar.gz https://go.dev/dl/go1.21.4.src.tar.gz
```

### Verbose progress output

```bash
grab download https://go.dev/dl/go1.21.5.darwin-amd64.tar.gz --verbose
```

Example output:

```
Downloading: [======================================= ]  99.34% (135728945/136421772 bytes)
Downloaded: go1.21.5.darwin-amd64.tar.gz (size: 136421772 bytes)
```

### GitHub releases

```bash
grab download https://github.com/golang/go/archive/refs/tags/go1.21.5.tar.gz
```

## Hash

Compute file hashes to verify integrity.

```bash
# SHA-256 (default)
grab hash myfile.tar.gz --type sha256

# MD5
grab hash myfile.tar.gz --type md5

# SHA-1
grab hash myfile.tar.gz --type sha1
```

### Download and verify workflow

```bash
grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
grab hash main.zip --type sha256
```

## Help

```bash
grab --help
grab download --help
grab hash --help
```
