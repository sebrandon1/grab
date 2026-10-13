package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
)

var output string

const progressBarLen = 40

var verbose bool
var retries int
var timeout time.Duration

var downloadCmd = &cobra.Command{
	Use:   "download [url]...",
	Short: "Download files from URLs",
	Long: `Download files from one or more URLs to the current directory.

The download command fetches files from HTTP/HTTPS URLs and saves them locally.
File names are automatically determined from the Content-Disposition header
or extracted from the URL path. Files are saved to the current working directory.

Multiple URLs can be downloaded concurrently by providing multiple arguments.
Use the --verbose flag to see download progress with a real-time progress bar.`,
	Example: `  # Download a single file
  grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip

  # Download multiple files concurrently
  grab download https://go.dev/dl/go1.21.5.src.tar.gz https://go.dev/dl/go1.21.4.src.tar.gz

  # Download with verbose progress output
  grab download https://go.dev/dl/go1.21.5.darwin-amd64.tar.gz --verbose

  # Download from a GitHub release
  grab download https://github.com/golang/go/archive/refs/tags/go1.21.5.tar.gz

  # Download and verify a SHA-256 checksum
  grab download --sha256 <64-char-hex> https://example.com/file.tar.gz

  # Multiple files with progress tracking
  grab download -v https://go.dev/dl/go1.21.5.src.tar.gz https://go.dev/dl/go1.20.12.src.tar.gz`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(runDownload(cmd, args, verbose))
	},
}

func runDownload(cmd *cobra.Command, args []string, verbose bool) int {
	ctx := cmd.Context()
	out, _ := cmd.Flags().GetString("output")
	sumHex, _ := cmd.Flags().GetString("sha256")

	if r, err := cmd.Flags().GetInt("retries"); err == nil && r > 0 {
		lib.DefaultClient.RetryLimit = r
		lib.DefaultClient.RetryDelay = time.Second
	}

	if timeout > 0 {
		lib.DefaultClient.SetTimeout(timeout)
	}

	// Checksum-verified download: single URL only, manual request so the
	// checksum can be attached (GetBatch does not support per-request options).
	if sumHex != "" {
		return runChecksumDownload(ctx, args, verbose, out, sumHex)
	}

	// For a single URL with a specific output file path (not an existing directory),
	// bypass GetBatch and write directly to the requested path.
	if out != "" && len(args) == 1 {
		if fi, statErr := os.Stat(out); statErr != nil || !fi.IsDir() {
			return downloadSingle(ctx, out, args[0], nil, verbose)
		}
	}

	dst := "."
	if out != "" {
		dst = out
	}

	respCh, err := lib.GetBatch(ctx, 0, dst, args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	multiFile := len(args) > 1
	var wg sync.WaitGroup
	var failed atomic.Int32

	for resp := range respCh {
		resp := resp
		wg.Add(1)
		go func() {
			defer wg.Done()

			switch {
			case verbose && !multiFile:
				t := time.NewTicker(100 * time.Millisecond)
				progressDone := make(chan struct{})
				goroutineDone := make(chan struct{})
				go func() {
					defer close(goroutineDone)
					defer t.Stop()
					var lastCompleted int64
					for {
						select {
						case <-progressDone:
							return
						case <-t.C:
							size := resp.Size()
							completed := resp.BytesComplete()
							if completed == lastCompleted {
								continue
							}
							lastCompleted = completed
							if size > 0 {
								percent := float64(completed) / float64(size) * 100
								filledLen := int(float64(progressBarLen) * float64(completed) / float64(size))
								bar := "[" + strings.Repeat("=", filledLen) + strings.Repeat(" ", progressBarLen-filledLen) + "]"
								fmt.Printf("\rDownloading: %s %6.2f%% (%d/%d bytes)", bar, percent, completed, size)
							} else {
								fmt.Printf("\rDownloading: %d bytes complete", completed)
							}
						}
					}
				}()
				<-resp.Done
				close(progressDone)
				<-goroutineDone
				fmt.Println()
			case verbose && multiFile:
				fmt.Printf("Downloading %s...\n", resp.Filename)
				<-resp.Done
			default:
				<-resp.Done
			}

			if err := resp.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed: %s (%v)\n", resp.Filename, err)
				failed.Add(1)
			} else if verbose {
				fmt.Printf("Downloaded: %s (size: %d bytes)\n", resp.Filename, resp.BytesComplete())
			}
		}()
	}

	wg.Wait()
	return int(failed.Load())
}

func runChecksumDownload(ctx context.Context, args []string, verbose bool, out, sumHex string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "--sha256 requires exactly one URL")
		return 1
	}
	sum, err := hex.DecodeString(sumHex)
	if err != nil || len(sum) != sha256.Size {
		fmt.Fprintf(os.Stderr, "--sha256 must be a %d-character hex string\n", 2*sha256.Size)
		return 1
	}
	return downloadSingle(ctx, out, args[0], sum, verbose)
}

func downloadSingle(ctx context.Context, dst, url string, sum []byte, verbose bool) int {
	req, err := lib.NewRequest(dst, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if sum != nil {
		req.SetChecksum(sha256.New(), sum, true)
	}
	req = req.WithContext(ctx)

	resp := lib.DefaultClient.Do(req)
	if verbose {
		fmt.Printf("Downloading %s...\n", url)
	}
	<-resp.Done
	if err := resp.Err(); err != nil {
		if errors.Is(err, lib.ErrBadChecksum) {
			fmt.Fprintf(os.Stderr, "Failed: %s (checksum mismatch; file deleted, verify the --sha256 value)\n", resp.Filename)
		} else {
			fmt.Fprintf(os.Stderr, "Failed: %s (%v)\n", resp.Filename, err)
		}
		return 1
	}
	if verbose {
		verified := ""
		if sum != nil {
			verified = ", sha256 verified"
		}
		fmt.Printf("Downloaded: %s (size: %d bytes%s)\n", resp.Filename, resp.BytesComplete(), verified)
	}
	return 0
}

func init() {
	downloadCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with real-time progress bar and download details")
	downloadCmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (single URL) or destination directory (multiple URLs)")
	downloadCmd.Flags().IntVar(&retries, "retries", 0, "Number of retry attempts for transient HTTP errors (5xx, 429)")
	downloadCmd.Flags().DurationVar(&timeout, "timeout", 0, "Network phase timeout (dial, TLS handshake, response headers), e.g. 30s (0 = no timeout)")
	downloadCmd.Flags().String("sha256", "", "Verify the download against a hex-encoded SHA-256 checksum; on mismatch the file is deleted (single URL only)")
	rootCmd.AddCommand(downloadCmd)
}
