package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
)

var verbose bool

var downloadCmd = &cobra.Command{
	Use:   "download [url]...",
	Short: "Download files from URLs",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := lib.NewClient()
		failed := 0
		for _, url := range args {
			req, err := lib.NewRequest(".", url)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid URL: %s (%v)\n", url, err)
				failed++
				continue
			}
			resp := client.Do(req)
			if verbose {
				t := time.NewTicker(100 * time.Millisecond)
				defer t.Stop()
				progressDone := make(chan struct{})
				go func() {
					for {
						select {
						case <-progressDone:
							return
						case <-t.C:
							size := resp.Size()
							completed := resp.BytesComplete()
							if size > 0 {
								percent := float64(completed) / float64(size) * 100
								barLen := 40
								filledLen := int(float64(barLen) * float64(completed) / float64(size))
								bar := "[" + strings.Repeat("=", filledLen) + strings.Repeat(" ", barLen-filledLen) + "]"
								fmt.Printf("\rDownloading: %s %6.2f%% (%d/%d bytes)", bar, percent, completed, size)
							} else {
								fmt.Printf("\rDownloading: %d bytes complete", completed)
							}
						}
					}
				}()
				<-resp.Done
				close(progressDone)
				fmt.Println() // Newline after progress bar
			} else {
				<-resp.Done
			}
			if verbose {
				if err := resp.Err(); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Failed: %s (%v)\n", resp.Filename, err)
				} else {
					info := ""
					if fi, err := os.Stat(resp.Filename); err == nil {
						size := fi.Size()
						info += fmt.Sprintf("size: %d bytes", size)
					}
					_, _ = fmt.Fprintf(os.Stdout, "Downloaded: %s (%s)\n", resp.Filename, info)
				}
			}
			if resp.Err() != nil {
				failed++
			}
		}
		os.Exit(failed)
	},
}

func init() {
	downloadCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.AddCommand(downloadCmd)
}
