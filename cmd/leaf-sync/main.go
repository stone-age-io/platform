// Command leaf-sync is the Stone Age platform's edge agent. It runs alongside a
// stock NATS leaf node on an edge box and mirrors its organization's PocketBase
// config into the leaf's local JetStream KV.
//
//	leaf-sync config       # bootstrap nats-leaf.conf + creds from PocketBase
//	leaf-sync run          # daemon: mirror config collections into local KV
//	leaf-sync run --nats   # ...and run the leaf node itself, in this process
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"platform/internal/leafsync"
	"platform/internal/version"
)

func main() {
	var cfgPath string

	root := &cobra.Command{
		Use:   "leaf-sync",
		Short: "Edge agent: mirror central PocketBase config into local NATS KV",
		// Cobra wires up `--version` automatically when this is set.
		Version: version.Version,
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

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the sync daemon (PocketBase -> local NATS KV)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := leafsync.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			// Flags beat the file and the environment, so only apply the ones
			// actually given — an unset bool flag is false and would otherwise
			// silently turn off nats.embedded: true from leaf-sync.yaml.
			if cmd.Flags().Changed("nats") {
				cfg.EmbedNATS, _ = cmd.Flags().GetBool("nats")
			}
			if cmd.Flags().Changed("nats-config") {
				cfg.EmbeddedConfig, _ = cmd.Flags().GetString("nats-config")
			}
			return leafsync.Run(cmd.Context(), cfg)
		},
	}
	// Same names as the Control Plane's `serve --nats` / `--nats-config`, doing
	// the same job one tier down. Equivalent to nats.embedded / nats.embedded_config
	// in leaf-sync.yaml (or LEAF_SYNC_NATS_EMBEDDED).
	runCmd.Flags().Bool("nats", false,
		"run the NATS leaf node in this process, from the nats-leaf.conf written by `leaf-sync config`")
	runCmd.Flags().String("nats-config", "",
		"path to the nats-leaf.conf used by --nats (default: <output.dir>/"+leafsync.LeafConfName+")")
	root.AddCommand(runCmd)

	// Cancel the command context on SIGINT/SIGTERM so `run` shuts down cleanly
	// and any in-flight PocketBase/NATS calls are cancelled.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "leaf-sync:", err)
		os.Exit(1)
	}
}
