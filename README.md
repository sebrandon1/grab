# grab

A minimal Go CLI and library for downloading files from the internet, inspired by cURL and wget.

## Features

- Download files from URLs to your local directory
- Guess filename from content header or URL path
- Download batches of files concurrently
- Simple, modern CLI with `grab download [url]...`
- Usable as a Go library or standalone binary

## Requirements

- Go 1.23 or newer
- macOS, Linux, or Windows

## Install

```
git clone https://github.com/sebrandon1/grab.git
cd grab
make build
```

This will produce a `grab` binary in the repo root.

## Usage

### CLI

Download a file:

```
./grab download https://example.com/file.zip
```

Download multiple files:

```
./grab download https://example.com/file1.zip https://example.com/file2.zip
```

Enable verbose output (shows a progress bar):

```
$ ./grab download https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.18.3/openshift-install-mac.tar.gz -v
Downloading: [======================================= ]  99.34% (429784057/432634124 bytes)
Downloaded: openshift-install-mac.tar.gz (size: 432634124 bytes)
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
	urls := []string{"https://example.com/file.zip"}
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
