package leafsync

import (
	"fmt"
	"os"

	"platform/internal/natsd"
)

// startEmbeddedNATS runs the edge's leaf node inside the leaf-sync process,
// loading the same nats-leaf.conf that `leaf-sync config` writes.
//
// It reuses internal/natsd, the wrapper the Control Plane's `serve --nats` uses,
// and inherits its two useful properties: it is an ordinary nats-server reading
// an ordinary config file (so splitting the two back apart is a flag, not a
// migration), and it refuses to return a server that started but reported a
// fatal — a leaf whose JetStream silently failed to open is worse than one that
// did not start, because the config mirror then writes into nothing.
//
// What it buys on an edge box: one process to install and supervise instead of
// two, and the end of the failure where `leaf-sync config` regenerates
// nats-leaf.conf and nobody restarts the server reading it.
//
// It is not the default. Where an init system is already managing services, a
// separately supervised nats-server is still the better shape — the bus then
// survives a leaf-sync restart, which is exactly what you want if you are
// upgrading the agent on a live site.
func startEmbeddedNATS(cfg *Config) (*natsd.Server, error) {
	// natsd has its own not-found message, but it names `stone-age nats export`
	// — the Control Plane's command, which does not produce this file. Check
	// here so the edge is pointed at the command that does.
	if _, err := os.Stat(cfg.EmbeddedConfig); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"leaf config not found at %s\n"+
					"       Bootstrap it first:  leaf-sync config\n"+
					"       Or point at an existing one with --nats-config",
				cfg.EmbeddedConfig)
		}
		return nil, fmt.Errorf("cannot read leaf config %s: %w", cfg.EmbeddedConfig, err)
	}

	// LocalNatsURL is passed so natsd can check it against the port the server
	// actually binds. Getting those two out of step leaves this process dialling
	// a leaf that only it is hosting, and failing.
	return natsd.Start(cfg.EmbeddedConfig, cfg.LocalNatsURL, "nats.local_url")
}
