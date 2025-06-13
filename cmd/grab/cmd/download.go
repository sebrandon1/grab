package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sebrandon1/grab/lib"
	"github.com/spf13/cobra"
)

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
			if resp.Err != nil {
				failed++
			}
		}
		os.Exit(failed)
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
