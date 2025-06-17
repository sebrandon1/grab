package cmd

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "grab",
	Short: "Download files from URLs",
	Long:  `grab downloads files from the provided URLs to the current directory`,
}

var hashCmd = &cobra.Command{
	Use:   "hash [file]",
	Short: "Compute and print the hash of a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		hashType, _ := cmd.Flags().GetString("type")
		f, err := os.Open(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			_ = f.Close()
		}()
		var sum []byte
		switch strings.ToLower(hashType) {
		case "sha256":
			hash := sha256.New()
			if _, err := io.Copy(hash, f); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to hash file: %v\n", err)
				os.Exit(1)
			}
			sum = hash.Sum(nil)
		case "sha1":
			hash := sha1.New()
			if _, err := io.Copy(hash, f); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to hash file: %v\n", err)
				os.Exit(1)
			}
			sum = hash.Sum(nil)
		case "md5":
			hash := md5.New()
			if _, err := io.Copy(hash, f); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to hash file: %v\n", err)
				os.Exit(1)
			}
			sum = hash.Sum(nil)
		default:
			fmt.Fprintf(os.Stderr, "Unknown hash type: %s\n", hashType)
			os.Exit(1)
		}
		fmt.Printf("%s  %s\n", hex.EncodeToString(sum), file)
	},
}

func init() {
	hashCmd.Flags().StringP("type", "t", "sha256", "Hash type: sha256, sha1, md5")
	rootCmd.AddCommand(hashCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
