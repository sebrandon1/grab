package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

var output string

const progressBarLen = 40

// rateLimitBurst is the shared limiter's token-bucket burst: a download can
// exceed the configured rate by up to this amount before pacing begins.
const rateLimitBurst = 64 * 1024

// rateLimitBufferSize is the transfer buffer size used while rate limiting.
// A small buffer keeps RateLimiter waits frequent, smoothing the pace and
// keeping progress updates live. It must not exceed rateLimitBurst, or a
// single read can request more tokens than the bucket can dispense.
const rateLimitBufferSize = 4 * 1024

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

  # Cap download rate at 200 KB/s
  grab download --limit-rate 200k https://go.dev/dl/go1.21.5.src.tar.gz

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
	limitRate, _ := cmd.Flags().GetString("limit-rate")

	if r, err := cmd.Flags().GetInt("retries"); err == nil && r > 0 {
		lib.DefaultClient.RetryLimit = r
		lib.DefaultClient.RetryDelay = time.Second
	}

	if timeout > 0 {
		lib.DefaultClient.SetTimeout(timeout)
	}

	// One shared limiter caps aggregate throughput across all concurrent
	// transfers in this invocation.
	var limiter *rate.Limiter
	if limitRate != "" {
		n, limitErr := parseByteRate(limitRate)
		if limitErr != nil {
			fmt.Fprintf(os.Stderr, "invalid --limit-rate value %q: %v\n", limitRate, limitErr)
			return 1
		}
		limiter = rate.NewLimiter(rate.Limit(n), rateLimitBurst)
	}

	// Checksum-verified download: single URL only, manual request so the
	// checksum can be attached (GetBatch does not support per-request options).
	if sumHex != "" {
		return runChecksumDownload(ctx, args, verbose, out, sumHex, limiter)
	}

	// For a single URL with a specific output file path (not an existing directory),
	// bypass GetBatch and write directly to the requested path.
	if out != "" && len(args) == 1 {
		if fi, statErr := os.Stat(out); statErr != nil || !fi.IsDir() {
			return downloadSingle(ctx, out, args[0], nil, limiter, verbose)
		}
	}

	dst := "."
	if out != "" {
		dst = out
	}

	// Requests are built manually so per-request options (the shared rate
	// limiter) can be attached; GetBatch does not support them. This mirrors
	// GetBatch's contract that dst is an existing directory.
	fi, statErr := os.Stat(dst)
	if statErr != nil {
		fmt.Fprintln(os.Stderr, statErr)
		return 1
	}
	if !fi.IsDir() {
		fmt.Fprintln(os.Stderr, "destination is not a directory")
		return 1
	}

	reqs := make([]*lib.Request, 0, len(args))
	for _, u := range args {
		req, reqErr := lib.NewRequest(dst, u)
		if reqErr != nil {
			fmt.Fprintln(os.Stderr, reqErr)
			return 1
		}
		applyRateLimit(req, limiter)
		reqs = append(reqs, req)
	}
	respCh := lib.DefaultClient.DoBatch(ctx, 0, reqs...)

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

func runChecksumDownload(ctx context.Context, args []string, verbose bool, out, sumHex string, limiter *rate.Limiter) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "--sha256 requires exactly one URL")
		return 1
	}
	sum, err := hex.DecodeString(sumHex)
	if err != nil || len(sum) != sha256.Size {
		fmt.Fprintf(os.Stderr, "--sha256 must be a %d-character hex string\n", 2*sha256.Size)
		return 1
	}
	return downloadSingle(ctx, out, args[0], sum, limiter, verbose)
}

func downloadSingle(ctx context.Context, dst, url string, sum []byte, limiter *rate.Limiter, verbose bool) int {
	req, err := lib.NewRequest(dst, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if sum != nil {
		req.SetChecksum(sha256.New(), sum, true)
	}
	applyRateLimit(req, limiter)
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

// parseByteRate parses a wget-style byte rate: plain bytes, or a k/K (×1024)
// or m/M (×1024²) suffix.
func parseByteRate(s string) (int64, error) {
	mult := int64(1)
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 'k', 'K':
			mult = 1024
			s = s[:len(s)-1]
		case 'm', 'M':
			mult = 1024 * 1024
			s = s[:len(s)-1]
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, fmt.Errorf("value out of range")
		}
		return 0, fmt.Errorf("must be a number with an optional k or m suffix, e.g. 200k")
	}
	if n > math.MaxInt64/mult {
		return 0, fmt.Errorf("value out of range")
	}
	n *= mult
	if n <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return n, nil
}

// applyRateLimit attaches the shared limiter to req.
func applyRateLimit(req *lib.Request, limiter *rate.Limiter) {
	if limiter == nil {
		return
	}
	req.RateLimiter = limiter
	req.BufferSize = rateLimitBufferSize
}

func init() {
	downloadCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with real-time progress bar and download details")
	downloadCmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (single URL) or destination directory (multiple URLs)")
	downloadCmd.Flags().IntVar(&retries, "retries", 0, "Number of retry attempts for transient HTTP errors (5xx, 429)")
	downloadCmd.Flags().DurationVar(&timeout, "timeout", 0, "Network phase timeout (dial, TLS handshake, response headers), e.g. 30s (0 = no timeout)")
	downloadCmd.Flags().String("sha256", "", "Verify the download against a hex-encoded SHA-256 checksum; on mismatch the file is deleted (single URL only)")
	downloadCmd.Flags().String("limit-rate", "", "Limit download rate, e.g. 200k or 2m (caps aggregate throughput across concurrent downloads)")
	rootCmd.AddCommand(downloadCmd)
}
