package leafsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"platform/internal/leafsync/pbclient"
)

// allowedCollections is the hard allowlist of config collections a leaf node may
// mirror. It mirrors the server-side API-rule grants; secret-bearing collections
// (nats_*, nebula_*) are intentionally excluded and can never be synced even if
// they somehow appear in a leaf node's synced_collections.
var allowedCollections = map[string]bool{
	"things":         true,
	"locations":      true,
	"thing_types":    true,
	"location_types": true,
}

const listPageSize = 500 // PocketBase per-page maximum

// Run authenticates to PocketBase as the leaf node, connects to the local leaf,
// and reconciles the configured collections into local KV on an interval until
// ctx is cancelled (e.g. on SIGINT/SIGTERM).
func Run(ctx context.Context, cfg *Config) error {
	pb := pbclient.New(cfg.PocketBaseURL)
	leaf, err := pb.AuthWithPassword(ctx, "leaf_nodes", cfg.PocketBaseEmail, cfg.PocketBasePassword)
	if err != nil {
		return fmt.Errorf("authenticate to PocketBase: %w", err)
	}

	collections := resolveCollections(leaf)
	if len(collections) == 0 {
		log.Printf("⚠️ leaf-sync: no syncable collections configured for this leaf node; nothing to do")
	} else {
		log.Printf("leaf-sync: mirroring %v every %s", collections, cfg.SyncInterval)
	}

	nc, err := nats.Connect(cfg.LocalNatsURL,
		nats.UserCredentials(cfg.CredsFile),
		nats.Name("leaf-sync"),
	)
	if err != nil {
		return fmt.Errorf("connect to local NATS (%s): %w", cfg.LocalNatsURL, err)
	}
	defer nc.Close()

	kw, err := newKVWriter(nc)
	if err != nil {
		return fmt.Errorf("init JetStream: %w", err)
	}

	// Run once immediately, then on the ticker until cancelled.
	syncAll(ctx, pb, kw, collections)

	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("leaf-sync: shutdown signal received, stopping")
			return nil
		case <-ticker.C:
			syncAll(ctx, pb, kw, collections)
		}
	}
}

// resolveCollections reads the leaf node's synced_collections, intersects it with
// the hard allowlist, and de-duplicates (a collection listed twice is synced once).
func resolveCollections(leaf pbclient.Record) []string {
	raw, ok := leaf["synced_collections"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, v := range arr {
		s, ok := v.(string)
		if !ok || !allowedCollections[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func syncAll(ctx context.Context, pb *pbclient.Client, kw *kvWriter, collections []string) {
	for _, col := range collections {
		if ctx.Err() != nil {
			return // cancelled; stop promptly
		}
		if err := syncCollection(ctx, pb, kw, col); err != nil {
			// Fail-soft: keep local KV as-is and retry next interval.
			log.Printf("leaf-sync: sync %q failed (will retry): %v", col, err)
		}
	}
}

// syncCollection performs a full reconcile of one collection: upsert every record
// fetched from PocketBase, then purge any KV key whose record no longer exists.
func syncCollection(ctx context.Context, pb *pbclient.Client, kw *kvWriter, col string) error {
	kv, err := kw.bucket(ctx, col)
	if err != nil {
		return fmt.Errorf("kv bucket: %w", err)
	}

	fetched := make(map[string]bool)
	for page := 1; ; page++ {
		res, err := pb.List(ctx, col, page, listPageSize, "")
		if err != nil {
			return err
		}
		for _, rec := range res.Items {
			id, _ := rec["id"].(string)
			if id == "" {
				continue
			}
			payload, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			if _, err := kv.Put(ctx, id, payload); err != nil {
				log.Printf("leaf-sync: kv put %s/%s: %v", col, id, err)
				continue
			}
			fetched[id] = true
		}
		if res.TotalPages == 0 || res.Page >= res.TotalPages {
			break
		}
	}

	// Reconcile deletions: remove KV keys for records that no longer exist.
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil
		}
		return fmt.Errorf("kv keys: %w", err)
	}
	for _, k := range keysToDelete(keys, fetched) {
		if err := kv.Delete(ctx, k); err != nil {
			log.Printf("leaf-sync: kv delete %s/%s: %v", col, k, err)
		}
	}
	return nil
}

// keysToDelete returns the KV keys that should be purged: those present locally
// but absent from the latest authoritative fetch from PocketBase.
func keysToDelete(existing []string, fetched map[string]bool) []string {
	var stale []string
	for _, k := range existing {
		if !fetched[k] {
			stale = append(stale, k)
		}
	}
	return stale
}
