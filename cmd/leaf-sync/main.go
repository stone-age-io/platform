// Command leaf-sync is the Stone Age platform's edge agent. It runs alongside a
// stock NATS leaf node on an edge box and mirrors its organization's PocketBase
// config into the leaf's local JetStream KV.
//
//	leaf-sync config   # bootstrap nats-leaf.conf + creds from PocketBase
//	leaf-sync run      # daemon: mirror config collections into local KV
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"platform/internal/leafsync"
)

func main() {
	var cfgPath string

	root := &cobra.Command{
		Use:   "leaf-sync",
		Short: "Edge agent: mirror central PocketBase config into local NATS KV",
		// We print errors ourselves below; don't let cobra dump usage on a
		// runtime (non-flag) failure or duplicate the error message.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "Path to leaf-sync.yaml")

	root.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Bootstrap the local NATS leaf config (nats-leaf.conf + creds) from PocketBase",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := leafsync.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			return leafsync.Bootstrap(cmd.Context(), cfg)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the sync daemon (PocketBase -> local NATS KV)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := leafsync.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			return leafsync.Run(cmd.Context(), cfg)
		},
	})

	// Cancel the command context on SIGINT/SIGTERM so `run` shuts down cleanly
	// and any in-flight PocketBase/NATS calls are cancelled.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "leaf-sync:", err)
		os.Exit(1)
	}
}
