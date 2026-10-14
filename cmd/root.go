package cmd

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

Supports multiple hash algorithms including MD5, SHA1, SHA256 (default), and SHA512.
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
		os.Exit(runHash(cmd, args))
	},
}

func runHash(cmd *cobra.Command, args []string) int {
	file := args[0]
	hashType, _ := cmd.Flags().GetString("type")
	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
		return 1
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
			return 1
		}
		sum = hash.Sum(nil)
	case "sha1":
		hash := sha1.New()
		if _, err := io.Copy(hash, f); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to hash file: %v\n", err)
			return 1
		}
		sum = hash.Sum(nil)
	case "sha512":
		hash := sha512.New()
		if _, err := io.Copy(hash, f); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to hash file: %v\n", err)
			return 1
		}
		sum = hash.Sum(nil)
	case "md5":
		hash := md5.New()
		if _, err := io.Copy(hash, f); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to hash file: %v\n", err)
			return 1
		}
		sum = hash.Sum(nil)
	default:
		fmt.Fprintf(os.Stderr, "Unknown hash type: %s\n", hashType)
		return 1
	}
	fmt.Printf("%s  %s\n", hex.EncodeToString(sum), file)
	return 0
}

func init() {
	hashCmd.Flags().StringP("type", "t", "sha256", "Hash algorithm to use (sha256, sha1, sha512, md5)")
	rootCmd.AddCommand(hashCmd)
}

// SetVersionInfo wires build metadata into the root command's --version
// output. Cobra only registers the --version flag once Version is non-empty,
// so this must be called before Execute.
func SetVersionInfo(version, commit, date string) {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("%s version %s (commit %s, built %s)\n", rootCmd.Name(), version, commit, date))
}

// shutdownSignals lists the signals that cancel in-flight downloads. Windows
// never delivers SIGTERM, but the constant compiles on every platform, so no
// build tags are needed.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func Execute() {
	// Cancel in-flight downloads on SIGINT/SIGTERM so each response finalizes
	// with a per-file error and keeps its partial file on disk, instead of the
	// signal killing the process mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()
	// Restore the default signal disposition once the first signal arrives so
	// a second Ctrl-C terminates the process instead of being swallowed while
	// downloads finalize.
	go func() {
		<-ctx.Done()
		stop()
	}()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
