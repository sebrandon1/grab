//go:build !windows

package cmd

import (
	"context"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// Each signal in shutdownSignals must cancel a signal.NotifyContext built
// from it, proving the Execute wiring delivers graceful cancellation for
// both SIGINT and SIGTERM.
func TestShutdownSignals_CancelContext(t *testing.T) {
	for _, sig := range shutdownSignals {
		t.Run(sig.String(), func(t *testing.T) {
			ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
			defer stop()
			unixSig, ok := sig.(syscall.Signal)
			if !ok {
				t.Fatalf("signal %v is not a syscall.Signal", sig)
			}
			if err := syscall.Kill(syscall.Getpid(), unixSig); err != nil {
				t.Fatalf("failed to deliver %v: %v", sig, err)
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatalf("expected context cancellation on %v", sig)
			}
		})
	}
}
