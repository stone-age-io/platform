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
	// Start the bus first, before PocketBase is involved at all. A site whose
	// uplink is down must still come up with a working local NATS — that
	// autonomy is the point of a leaf node, and the separate-process topology
	// gets it for free by not sequencing the two.
	if cfg.EmbedNATS {
		srv, err := startEmbeddedNATS(cfg)
		if err != nil {
			return err
		}
		defer srv.Stop()
	}

	pb := pbclient.New(cfg.PocketBaseURL)
	leaf, err := authenticate(ctx, pb, cfg)
	if err != nil {
		if ctx.Err() != nil {
			log.Printf("leaf-sync: shutdown signal received, stopping")
			return nil
		}
		return err
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

	// Optional data plane: mirror `twin_desired` down from the hub and relay
	// `twin` up to it. Independent of the config cycle below, and disables itself
	// (logging why) rather than failing the agent — config sync must survive a
	// data-plane problem.
	startTwin(ctx, nc, cfg)

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

// Backoff bounds for the PocketBase login retry below. Variables rather than
// constants only so the tests can shrink them; nothing in production reassigns
// them.
var (
	authRetryMin = 2 * time.Second
	authRetryMax = 60 * time.Second
)

// authenticate logs in to PocketBase as the leaf node.
//
// Without --nats it fails on the first error, which is the long-standing
// behaviour: the bus is another process, so exiting and letting a supervisor
// restart us costs nothing.
//
// With --nats it retries instead, because exiting would stop the leaf server
// too. A supervisor restarting the pair every few seconds through a WAN outage
// means devices reconnecting and JetStream recovering its store on a loop — the
// site's local messaging broken by the central platform being unreachable,
// which is precisely the coupling a leaf node exists to avoid. Stale config is
// the correct thing to serve meanwhile; that is what the config mirror is for.
//
// It retries on any failure, a rejected password included, rather than trying to
// classify which errors are transient. A typo in the config should not take a
// working bus offline either, and the log names the cause on every attempt.
func authenticate(ctx context.Context, pb *pbclient.Client, cfg *Config) (pbclient.Record, error) {
	backoff := authRetryMin
	for {
		leaf, err := pb.AuthWithPassword(ctx, "leaf_nodes", cfg.PocketBaseEmail, cfg.PocketBasePassword)
		if err == nil {
			return leaf, nil
		}
		if !cfg.EmbedNATS {
			return nil, fmt.Errorf("authenticate to PocketBase: %w", err)
		}
		log.Printf("⚠️ leaf-sync: PocketBase auth failed (%v); local NATS is up, retrying in %s", err, backoff)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("authenticate to PocketBase: %w", err)
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, authRetryMax)
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
func syncAll(ctx context.Context, pb recordLister, kw *kvWriter, cache *syncCache, collections []string) (map[string]int, []string) {
	synced := make(map[string]int, len(collections))
	var errs []string
	for _, col := range collections {
		if ctx.Err() != nil {
			return synced, errs // cancelled; stop promptly
		}
		kv, err := kw.bucket(ctx, col)
		if err != nil {
			log.Printf("leaf-sync: kv bucket %q failed (will retry): %v", col, err)
			errs = append(errs, fmt.Sprintf("%s: kv bucket: %v", col, err))
			continue
		}
		n, err := syncCollection(ctx, pb, kv, cache, col)
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

// recordLister is the slice of pbclient.Client that syncCollection needs. Kept
// narrow so the reconcile logic can be driven by a fake in tests.
type recordLister interface {
	List(ctx context.Context, collection string, page, perPage int, filter string) (*pbclient.ListResult, error)
}

// kvBucket is the slice of jetstream.KeyValue that syncCollection needs. Same
// reason as recordLister; jetstream.KeyValue satisfies it as-is.
type kvBucket interface {
	Put(ctx context.Context, key string, value []byte) (uint64, error)
	Delete(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error
	Keys(ctx context.Context, opts ...jetstream.WatchOpt) ([]string, error)
}

// syncCollection performs a full reconcile of one collection: upsert every record
// fetched from PocketBase, then purge any KV key whose record no longer exists.
// It returns the number of records the collection should have in KV.
//
// The set of keys that SHOULD exist is derived only from what PocketBase returned
// — never from whether a write succeeded. Those two were conflated in one map
// before, which made a transient Put failure delete a live record from the edge:
// the failed key was missing from the map, so the deletion pass below read it as
// "no longer exists upstream" and purged it.
func syncCollection(ctx context.Context, pb recordLister, kv kvBucket, cache *syncCache, col string) (int, error) {
	// What the bucket holds right now, read BEFORE writing. It serves two
	// purposes: the deletion pass needs it, and it tells us which cached hashes
	// the store still corroborates (see the `present` check below).
	existing, err := kv.Keys(ctx)
	if err != nil && !errors.Is(err, jetstream.ErrNoKeysFound) {
		return 0, fmt.Errorf("kv keys: %w", err)
	}
	present := make(map[string]bool, len(existing))
	for _, k := range existing {
		present[k] = true
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
	if len(records) == 0 && len(existing) > 0 {
		log.Printf("⚠️ leaf-sync: %q returned 0 records but %d keys exist locally; skipping purge this cycle", col, len(existing))
		return 0, nil
	}

	// Count candidate handles so a code shared by two records falls back to id.
	counts := make(map[string]int)
	for _, rec := range records {
		if c := candidateKey(rec); c != "" {
			counts[c]++
		}
	}

	// Pass 1: choose each record's KV key. `desired` is the authoritative answer
	// to "which keys should this bucket hold", and nothing below removes from it.
	desired := make(map[string]bool, len(records))
	keyed := make([]keyedRecord, 0, len(records))
	for _, rec := range records {
		if id, _ := rec["id"].(string); id == "" {
			continue // unkeyable; PocketBase always sets id, so this is defensive
		}
		key := recordKey(rec, counts)
		desired[key] = true
		keyed = append(keyed, keyedRecord{key: key, rec: rec})
	}

	// Pass 2: write the records whose content actually changed.
	changed, failed := 0, 0
	for _, kr := range keyed {
		payload, err := json.Marshal(strip(kr.rec))
		if err != nil {
			log.Printf("leaf-sync: marshal %s/%s: %v", col, kr.key, err)
			failed++
			continue
		}
		// Changed-only write: skip the Put when this key's content is byte-for-byte
		// what we last wrote AND the bucket still actually holds the key.
		// json.Marshal of a map emits sorted keys, so an unchanged record hashes
		// identically cycle to cycle. Without this, a full reconcile re-Puts every
		// record every interval and each key rolls over its 5-revision history on
		// churn alone.
		//
		// The `present` half is what keeps the cache honest. On its own the cache
		// records only what this process last wrote, so a key that disappeared
		// out-of-band — bucket deleted and recreated, store lost, someone ran
		// `nats kv del` — was never re-Put, and the gap in the mirror lasted for
		// the lifetime of the daemon.
		sum := sha256.Sum256(payload)
		if cache.unchanged(col, kr.key, sum) && present[kr.key] {
			continue
		}
		if _, err := kv.Put(ctx, kr.key, payload); err != nil {
			// Leave the cache alone: it still describes what the bucket holds, and
			// the next cycle sees this content as changed and retries the write.
			log.Printf("leaf-sync: kv put %s/%s: %v", col, kr.key, err)
			failed++
			continue
		}
		cache.remember(col, kr.key, sum)
		changed++
	}
	if changed > 0 {
		log.Printf("leaf-sync: %s: wrote %d changed of %d records", col, changed, len(desired))
	}

	// Reconcile deletions: remove KV keys whose record no longer exists upstream.
	// Safe even when writes failed above — `desired` comes from the fetch, so a
	// key absent from it genuinely has no record behind it any more.
	for _, k := range keysToDelete(existing, desired) {
		if err := kv.Delete(ctx, k); err != nil {
			log.Printf("leaf-sync: kv delete %s/%s: %v", col, k, err)
			failed++
			continue
		}
		cache.forget(col, k) // key is gone; re-Put it if a record ever reuses it
	}

	// Surface partial failure so the heartbeat reports this collection as errored
	// rather than silently claiming a clean cycle.
	if failed > 0 {
		return len(desired), fmt.Errorf("%d of %d records failed to write", failed, len(desired))
	}
	return len(desired), nil
}

// keyedRecord pairs a fetched record with the KV key chosen for it, so the key is
// decided for every record before any write happens.
type keyedRecord struct {
	key string
	rec pbclient.Record
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
//
// It records what this process wrote, which is not the same thing as what the
// bucket holds. syncCollection therefore treats a cache hit as authoritative only
// when the bucket's key listing still shows the key — otherwise an out-of-band
// deletion would never be repaired.
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
