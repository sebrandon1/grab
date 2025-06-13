package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
)

var verbose bool

var downloadCmd = &cobra.Command{
	Use:   "download [url]...",
	Short: "Download files from URLs",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		failed := 0
		respch, err := lib.DownloadBatch(context.Background(), args)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		for resp := range respch {
			if verbose {
				if resp.Err != nil {
					fmt.Fprintf(os.Stderr, "Failed: %s (%v)\n", resp.Filename, resp.Err)
				} else {
					// Try to get more info if available
					info := ""
					if fi, err := os.Stat(resp.Filename); err == nil {
						size := fi.Size()
						info += fmt.Sprintf("size: %d bytes", size)
					}
					// Duration and speed are not available in DownloadResponse, so print only what we can
					fmt.Fprintf(os.Stdout, "Downloaded: %s (%s)\n", resp.Filename, info)
				}
			}
			if resp.Err != nil {
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
