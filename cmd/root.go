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
	Short: "A minimal CLI for downloading files from the internet",
	Long: `grab is a minimal Go CLI and library for downloading files from the internet, 
inspired by cURL and wget.

grab provides simple commands to download files from URLs, compute file hashes,
and more. Files are downloaded to the current working directory with automatic
filename detection from HTTP headers or URL paths.

Key features:
• Download single or multiple files concurrently
• Automatic filename detection from Content-Disposition headers
• Real-time progress tracking with verbose mode
• Built-in file hash computation (MD5, SHA1, SHA256)
• Clean, modern CLI interface`,
	Example: `  # Download a file
  grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip

  # Download with progress tracking
  grab download -v https://go.dev/dl/go1.21.5.src.tar.gz

  # Compute file hash
  grab hash myfile.zip --type sha256`,
}

var hashCmd = &cobra.Command{
	Use:   "hash [file]",
	Short: "Compute and print the hash of a file",
	Long: `Compute and print the cryptographic hash of a file.

Supports multiple hash algorithms including MD5, SHA1, and SHA256 (default).
Output format matches common checksum tools: hash followed by filename.`,
	Example: `  # Compute SHA256 hash (default)
  grab hash main.zip

  # Compute MD5 hash
  grab hash go1.21.5.src.tar.gz --type md5

  # Compute SHA1 hash
  grab hash go1.21.5.darwin-amd64.tar.gz -t sha1

  # Verify downloaded file integrity
  grab download https://github.com/sebrandon1/grab/archive/refs/heads/main.zip
  grab hash main.zip --type sha256`,
	Args: cobra.ExactArgs(1),
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
	hashCmd.Flags().StringP("type", "t", "sha256", "Hash algorithm to use (sha256, sha1, md5)")
	rootCmd.AddCommand(hashCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
