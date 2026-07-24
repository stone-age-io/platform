package leafsync

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"platform/internal/leafsync/pbclient"
)

// allowedCollections is the hard allowlist of config collections a leaf node may
// mirror. It mirrors the server-side API-rule grants; secret-bearing collections
// (nats_*, nebula_*) are intentionally excluded and can never be synced even if
// they somehow appear in a leaf node's synced_collections.
//
// Records are keyed in KV by the handle candidateKey derives (composite, code,
// then name), matching stone-cli's EntitySpec.LookupKey for these collections.
// Keep this set — and the key precedence in candidateKey — in step with
// stone-cli's cmd/entity.go specs.
var allowedCollections = map[string]bool{
	"things":                true,
	"locations":             true,
	"thing_types":           true,
	"location_types":        true,
	"thing_type_operations": true, // keyed by name; completes thing_type -> operation graph
	"message_schemas":       true, // keyed by namespace__name__version
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

	// Optional heartbeat: write liveness into the hub-domain leaf_status KV after
	// each cycle. Disabled (best-effort, never fatal) when hub_domain is unset or
	// the bucket can't be opened — the sync loop runs regardless.
	code, _ := leaf["code"].(string)
	hb := openHeartbeat(ctx, nc, cfg.HubDomain, code)

	// Remembers what was last written to each key so a reconcile only re-Puts
	// records that actually changed. Persists for the lifetime of this daemon.
	cache := newSyncCache()

	// One cycle: reconcile every collection, then publish a heartbeat.
	cycle := func() {
		synced, errs := syncAll(ctx, pb, kw, cache, collections)
		hb.publish(ctx, synced, errs, cfg.SyncInterval)
	}

	// Run once immediately, then on the ticker until cancelled.
	cycle()

	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("leaf-sync: shutdown signal received, stopping")
			return nil
		case <-ticker.C:
			cycle()
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

// syncAll reconciles every configured collection and returns the per-collection
// synced record count plus any errors, for the heartbeat payload. Fail-soft: a
// collection that errors is logged and recorded, local KV left as-is, and the
// remaining collections still run.
func syncAll(ctx context.Context, pb *pbclient.Client, kw *kvWriter, cache *syncCache, collections []string) (map[string]int, []string) {
	synced := make(map[string]int, len(collections))
	var errs []string
	for _, col := range collections {
		if ctx.Err() != nil {
			return synced, errs // cancelled; stop promptly
		}
		n, err := syncCollection(ctx, pb, kw, cache, col)
		if err != nil {
			// Fail-soft: keep local KV as-is and retry next interval.
			log.Printf("leaf-sync: sync %q failed (will retry): %v", col, err)
			errs = append(errs, fmt.Sprintf("%s: %v", col, err))
			continue
		}
		synced[col] = n
	}
	return synced, errs
}

// syncCollection performs a full reconcile of one collection: upsert every record
// fetched from PocketBase, then purge any KV key whose record no longer exists.
// It returns the number of records synced.
func syncCollection(ctx context.Context, pb *pbclient.Client, kw *kvWriter, cache *syncCache, col string) (int, error) {
	kv, err := kw.bucket(ctx, col)
	if err != nil {
		return 0, fmt.Errorf("kv bucket: %w", err)
	}

	// Fetch the whole collection before writing: KV keys prefer the record's
	// `code`, which is optional and non-unique in the schema, so we must see
	// every record to detect duplicate codes before choosing keys.
	var records []pbclient.Record
	for page := 1; ; page++ {
		res, err := pb.List(ctx, col, page, listPageSize, "")
		if err != nil {
			return 0, err
		}
		records = append(records, res.Items...)
		if res.TotalPages == 0 || res.Page >= res.TotalPages {
			break
		}
	}

	// Empty-fetch guard: a successful-but-empty response (e.g. a transient auth
	// or org-scoping glitch) must never purge an existing mirror. If we fetched
	// zero records but local KV still holds keys, leave it untouched this cycle.
	if len(records) == 0 {
		if keys, _ := kv.Keys(ctx); len(keys) > 0 {
			log.Printf("⚠️ leaf-sync: %q returned 0 records but %d keys exist locally; skipping purge this cycle", col, len(keys))
			return 0, nil
		}
	}

	// Count candidate handles so a code shared by two records falls back to id.
	counts := make(map[string]int)
	for _, rec := range records {
		if c := candidateKey(rec); c != "" {
			counts[c]++
		}
	}

	fetched := make(map[string]bool)
	changed := 0
	for _, rec := range records {
		if id, _ := rec["id"].(string); id == "" {
			continue
		}
		key := recordKey(rec, counts)
		payload, err := json.Marshal(strip(rec))
		if err != nil {
			continue
		}
		// Changed-only write: skip the Put when this key's content is byte-for-byte
		// what we last wrote. json.Marshal of a map emits sorted keys, so an
		// unchanged record hashes identically cycle to cycle. Without this, a full
		// reconcile re-Puts every record every interval, so each key rolls over its
		// 5-revision KV history on churn alone. The record is still marked fetched
		// so the deletion pass leaves it in place.
		sum := sha256.Sum256(payload)
		if cache.unchanged(col, key, sum) {
			fetched[key] = true
			continue
		}
		if _, err := kv.Put(ctx, key, payload); err != nil {
			log.Printf("leaf-sync: kv put %s/%s: %v", col, key, err)
			continue
		}
		cache.remember(col, key, sum)
		fetched[key] = true
		changed++
	}
	if changed > 0 {
		log.Printf("leaf-sync: %s: wrote %d changed of %d records", col, changed, len(fetched))
	}

	// Reconcile deletions: remove KV keys for records that no longer exist.
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return len(fetched), nil
		}
		return len(fetched), fmt.Errorf("kv keys: %w", err)
	}
	for _, k := range keysToDelete(keys, fetched) {
		if err := kv.Delete(ctx, k); err != nil {
			log.Printf("leaf-sync: kv delete %s/%s: %v", col, k, err)
			continue
		}
		cache.forget(col, k) // key is gone; re-Put it if a record ever reuses it
	}
	return len(fetched), nil
}

// serverOnlyFields are written by PocketBase and carry no value to a KV consumer.
// We strip the pure-noise ones (collectionId/collectionName/expand) but keep
// created/updated, which give edge consumers a useful freshness signal. This
// mirrors stone-cli's pb.ServerOnlyFields, minus the timestamps it drops only
// because `apply` would reject them.
var serverOnlyFields = []string{"collectionId", "collectionName", "expand"}

// strip removes server-only noise fields from a record in place and returns it.
func strip(rec pbclient.Record) pbclient.Record {
	for _, f := range serverOnlyFields {
		delete(rec, f)
	}
	return rec
}

// kvKeyRe is the set of characters NATS permits in a KV key.
var kvKeyRe = regexp.MustCompile(`^[-/_=\.a-zA-Z0-9]+$`)

// validKVKey reports whether s is usable as a NATS KV key (non-empty, not
// fenced by '.', and within the allowed character set). Codes that fail this
// fall back to the record id.
func validKVKey(s string) bool {
	if s == "" || s[0] == '.' || s[len(s)-1] == '.' {
		return false
	}
	return kvKeyRe.MatchString(s)
}

// candidateKey returns the human-facing handle for a record, following
// stone-cli's recordFilename precedence: a message_schema's composite identity
// (namespace__name__version) wins, then `code`, then `name`. An empty result
// means the record has no good handle and should be keyed by id.
func candidateKey(rec pbclient.Record) string {
	if ns, _ := rec["namespace"].(string); ns != "" {
		if nm, _ := rec["name"].(string); nm != "" {
			if v, _ := rec["version"].(string); v != "" {
				return ns + "__" + nm + "__" + v
			}
		}
	}
	if code, _ := rec["code"].(string); code != "" {
		return code
	}
	if name, _ := rec["name"].(string); name != "" {
		return name
	}
	return ""
}

// recordKey chooses the KV key for a record: its candidate handle when that
// handle is present, unique across the fetch (counts[c] == 1), and a valid KV
// key; otherwise the opaque-but-always-unique record id. The id stays inside
// the stored value either way, so id-based relation joins keep working.
func recordKey(rec pbclient.Record, counts map[string]int) string {
	id, _ := rec["id"].(string)
	if c := candidateKey(rec); c != "" && counts[c] == 1 && validKVKey(c) {
		return c
	}
	return id
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

// syncCache remembers, per collection, the content hash of the value last written
// to each KV key, so a reconcile only re-Puts records that actually changed. It
// lives for one `run` process; on restart the first cycle re-Puts everything once
// (cold cache) and then goes quiet. This is what keeps a bucket's 5-revision
// history from rolling over every interval when the underlying data is static.
type syncCache struct {
	seen map[string]map[string][32]byte // collection -> KV key -> sha256(payload)
}

func newSyncCache() *syncCache {
	return &syncCache{seen: make(map[string]map[string][32]byte)}
}

// unchanged reports whether sum matches the hash last remembered for (col, key).
// A miss — never seen, or a prior Put that failed and so was never remembered —
// returns false, so the caller writes.
func (c *syncCache) unchanged(col, key string, sum [32]byte) bool {
	prev, ok := c.seen[col][key] // reading a nil inner map is safe: ok == false
	return ok && prev == sum
}

// remember records sum as the latest value written for (col, key). Call it only
// after a successful Put, so a failed Put is retried next cycle.
func (c *syncCache) remember(col, key string, sum [32]byte) {
	m := c.seen[col]
	if m == nil {
		m = make(map[string][32]byte)
		c.seen[col] = m
	}
	m[key] = sum
}

// forget drops a key after it has been deleted from KV, so the entry doesn't leak
// and a future record that reuses the key is written afresh.
func (c *syncCache) forget(col, key string) {
	delete(c.seen[col], key) // deleting from a nil map is a no-op
}
