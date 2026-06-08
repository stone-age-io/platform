package leafsync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "leaf-sync.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeConfig(t, `
pocketbase:
  url: https://pb.example.com
  email: edge01@x.leaf.local
  password: secret
nats:
  hub_leaf_url: nats-leaf://hub:7422
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.LocalNatsURL != "nats://127.0.0.1:4222" {
		t.Errorf("default local_url not applied: %q", cfg.LocalNatsURL)
	}
	if cfg.CredsFile != "edge.creds" {
		t.Errorf("default creds_file not applied: %q", cfg.CredsFile)
	}
	if cfg.OutputDir != "." {
		t.Errorf("default output.dir not applied: %q", cfg.OutputDir)
	}
	if cfg.SyncInterval != 30*time.Second {
		t.Errorf("default sync.interval not applied: %v", cfg.SyncInterval)
	}
}

func TestLoadConfigRequiresPocketBaseFields(t *testing.T) {
	path := writeConfig(t, `
pocketbase:
  url: https://pb.example.com
  email: edge01@x.leaf.local
`) // password missing
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for missing pocketbase.password, got nil")
	}
}

func TestLoadConfigRejectsBadInterval(t *testing.T) {
	path := writeConfig(t, `
pocketbase:
  url: https://pb.example.com
  email: e
  password: p
sync:
  interval: "not-a-duration"
`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for invalid sync.interval, got nil")
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	path := writeConfig(t, `
pocketbase:
  url: https://pb.example.com
  email: e
  password: file-password
nats:
  hub_leaf_url: nats-leaf://hub:7422
`)
	t.Setenv("LEAF_SYNC_POCKETBASE_PASSWORD", "env-password")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PocketBasePassword != "env-password" {
		t.Errorf("env var did not override file value, got %q", cfg.PocketBasePassword)
	}
}
