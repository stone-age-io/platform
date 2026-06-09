package leafsync

import (
	"fmt"
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

	SyncInterval time.Duration

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
	v.SetDefault("output.dir", ".")
	v.SetDefault("sync.interval", "30s")
	v.SetDefault("jwt_refresh.enabled", false)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	interval, err := time.ParseDuration(v.GetString("sync.interval"))
	if err != nil {
		return nil, fmt.Errorf("invalid sync.interval: %w", err)
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
		SyncInterval:       interval,
		ReloadHook:         v.GetString("reload_hook"),
		JWTRefresh:         v.GetBool("jwt_refresh.enabled"),
	}

	if cfg.PocketBaseURL == "" || cfg.PocketBaseEmail == "" || cfg.PocketBasePassword == "" {
		return nil, fmt.Errorf("pocketbase.url, pocketbase.email and pocketbase.password are required")
	}

	return cfg, nil
}
