package leafsync

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the small, hand-carried configuration for a leaf-sync deployment.
// Everything else (domain, synced collections, NATS creds, operator/account
// JWTs) is pulled from central PocketBase at bootstrap/run time.
type Config struct {
	PocketBaseURL      string
	PocketBaseEmail    string
	PocketBasePassword string

	HubLeafURL   string // where the leaf node's remote dials (written into nats-leaf.conf)
	HubDomain    string // hub's JetStream domain; target for the liveness heartbeat (empty = off)
	LocalNatsURL string // the local leaf this agent connects to at run time
	CredsFile    string // path to the creds file (written by `config`, read by `run`)
	OutputDir    string // where `config` writes nats-leaf.conf + creds

	// EmbedNATS runs the leaf's nats-server inside this process rather than
	// alongside it, from the same nats-leaf.conf `config` writes. Off by default:
	// a separately supervised nats-server is still right wherever an init system
	// is already managing services, and it keeps the bus up across a leaf-sync
	// restart. See embedded.go and --nats.
	EmbedNATS bool

	// EmbeddedConfig is the nats-leaf.conf EmbedNATS loads. Empty means
	// <OutputDir>/nats-leaf.conf, which is where `config` put it.
	EmbeddedConfig string

	SyncInterval time.Duration

	// ObserveAddr is the listen address for /ready and /metrics. Empty (the
	// default) means the endpoints are not served — the readiness checks still
	// run and still log, they just are not reachable over the network. Opening
	// a port on an edge appliance should be a decision, not a default.
	ObserveAddr string
	// MetricsToken optionally protects /metrics. Empty means open, which is
	// reasonable on an address bound to loopback or a management LAN.
	MetricsToken string
	// MonitorURL is the local NATS server's monitoring endpoint — the `http:`
	// line `leaf-sync config` writes into nats-leaf.conf. Loopback and
	// unauthenticated by design, which is how the edge reads its own server's
	// state (uplink attached? JetStream size?) without ever holding a $SYS
	// identity. Empty disables the checks and metrics that depend on it.
	MonitorURL string
	// ReadinessInterval is how often the checks run.
	ReadinessInterval time.Duration

	// TwinEnabled turns on digital-twin sync between the local leaf domain and
	// the hub (see twin.go): a server-maintained mirror of `twin_desired` down,
	// and a relay of `twin` up. Off by default — it moves data plane traffic, so
	// an upgrade must not silently start doing it. Requires HubDomain.
	TwinEnabled bool

	// Reserved (off by default): optional account-JWT refresh + portable reload.
	ReloadHook string
	JWTRefresh bool
}

// LoadConfig resolves the leaf-sync config from a YAML file (or LEAF_SYNC_* env
// vars). Defaults are applied for everything except the required PocketBase
// connection fields.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("leaf-sync")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/leaf-sync/")
	}

	v.SetEnvPrefix("LEAF_SYNC")
	// Map nested config keys to flat env vars, e.g. pocketbase.password ->
	// LEAF_SYNC_POCKETBASE_PASSWORD. Without this, AutomaticEnv can't resolve
	// dotted keys and the documented env overrides silently do nothing.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("nats.local_url", "nats://127.0.0.1:4222")
	v.SetDefault("nats.creds_file", "edge.creds")
	v.SetDefault("nats.embedded", false)
	// Empty, not a path: the real default depends on output.dir, which isn't
	// resolved yet. Filled in below.
	v.SetDefault("nats.embedded_config", "")
	v.SetDefault("output.dir", ".")
	v.SetDefault("sync.interval", "30s")
	v.SetDefault("twin.enabled", false)
	v.SetDefault("jwt_refresh.enabled", false)
	// Matches the `http:` line buildLeafConf writes. Defaulted rather than left
	// empty because it is not a deployment choice on a config this tool
	// generated — it is the address of the file's own monitoring port.
	v.SetDefault("nats.monitor_url", "http://127.0.0.1:8222")
	v.SetDefault("observability.addr", "")
	v.SetDefault("observability.metrics_token", "")
	v.SetDefault("observability.interval", "15s")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	interval, err := time.ParseDuration(v.GetString("sync.interval"))
	if err != nil {
		return nil, fmt.Errorf("invalid sync.interval: %w", err)
	}

	observeInterval, err := time.ParseDuration(v.GetString("observability.interval"))
	if err != nil {
		return nil, fmt.Errorf("invalid observability.interval: %w", err)
	}

	cfg := &Config{
		PocketBaseURL:      v.GetString("pocketbase.url"),
		PocketBaseEmail:    v.GetString("pocketbase.email"),
		PocketBasePassword: v.GetString("pocketbase.password"),
		HubLeafURL:         v.GetString("nats.hub_leaf_url"),
		HubDomain:          v.GetString("nats.hub_domain"),
		LocalNatsURL:       v.GetString("nats.local_url"),
		CredsFile:          v.GetString("nats.creds_file"),
		OutputDir:          v.GetString("output.dir"),
		EmbedNATS:          v.GetBool("nats.embedded"),
		EmbeddedConfig:     v.GetString("nats.embedded_config"),
		SyncInterval:       interval,
		ObserveAddr:        v.GetString("observability.addr"),
		MetricsToken:       v.GetString("observability.metrics_token"),
		MonitorURL:         v.GetString("nats.monitor_url"),
		ReadinessInterval:  observeInterval,
		TwinEnabled:        v.GetBool("twin.enabled"),
		ReloadHook:         v.GetString("reload_hook"),
		JWTRefresh:         v.GetBool("jwt_refresh.enabled"),
	}

	// Point --nats at whatever `config` wrote, so the common case needs no second
	// path in the file. Resolved here rather than at use so that everything
	// downstream sees one, already-decided value.
	if cfg.EmbeddedConfig == "" {
		cfg.EmbeddedConfig = filepath.Join(cfg.OutputDir, LeafConfName)
	}

	if cfg.PocketBaseURL == "" || cfg.PocketBaseEmail == "" || cfg.PocketBasePassword == "" {
		return nil, fmt.Errorf("pocketbase.url, pocketbase.email and pocketbase.password are required")
	}

	return cfg, nil
}
