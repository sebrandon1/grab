package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
)

const progressBarLen = 40

var verbose bool

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

  # Multiple files with progress tracking
  grab download -v https://go.dev/dl/go1.21.5.src.tar.gz https://go.dev/dl/go1.20.12.src.tar.gz`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(runDownload(cmd, args, verbose))
	},
}

func runDownload(cmd *cobra.Command, args []string, verbose bool) int {
	ctx := cmd.Context()
	respCh, err := lib.GetBatch(ctx, 0, ".", args...)
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

func init() {
	downloadCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with real-time progress bar and download details")
	rootCmd.AddCommand(downloadCmd)
}
