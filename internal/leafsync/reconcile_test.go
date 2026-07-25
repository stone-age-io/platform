package leafsync

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"

	"platform/internal/leafsync/pbclient"
)

// --- fakes -------------------------------------------------------------------

// fakeLister serves canned pages for one collection and can be made to fail.
type fakeLister struct {
	records []pbclient.Record
	err     error
	calls   int
}

func (f *fakeLister) List(_ context.Context, _ string, page, perPage int, _ string) (*pbclient.ListResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	// Single page is enough for these tests; pagination is covered in pbclient.
	return &pbclient.ListResult{
		Page:       page,
		PerPage:    perPage,
		TotalItems: len(f.records),
		TotalPages: 1,
		Items:      f.records,
	}, nil
}

// fakeKV is an in-memory stand-in for a JetStream KV bucket. failPut names keys
// whose Put should fail; keysErr is returned by Keys.
type fakeKV struct {
	store   map[string][]byte
	failPut map[string]bool
	keysErr error

	puts    []string // every key Put was called with, in order
	deletes []string // every key Delete was called with, in order
}

func newFakeKV(seed map[string][]byte) *fakeKV {
	if seed == nil {
		seed = map[string][]byte{}
	}
	return &fakeKV{store: seed, failPut: map[string]bool{}}
}

func (f *fakeKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	f.puts = append(f.puts, key)
	if f.failPut[key] {
		return 0, errors.New("simulated put failure")
	}
	f.store[key] = value
	return 1, nil
}

func (f *fakeKV) Delete(_ context.Context, key string, _ ...jetstream.KVDeleteOpt) error {
	f.deletes = append(f.deletes, key)
	delete(f.store, key)
	return nil
}

func (f *fakeKV) Keys(_ context.Context, _ ...jetstream.WatchOpt) ([]string, error) {
	if f.keysErr != nil {
		return nil, f.keysErr
	}
	if len(f.store) == 0 {
		return nil, jetstream.ErrNoKeysFound
	}
	keys := make([]string, 0, len(f.store))
	for k := range f.store {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (f *fakeKV) storedKeys() []string {
	keys := make([]string, 0, len(f.store))
	for k := range f.store {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func rec(id, code string) pbclient.Record {
	return pbclient.Record{"id": id, "code": code, "name": "n-" + id}
}

// --- the two data-loss regressions ------------------------------------------

// A transient Put failure must not cause the key to be purged. Before the fix the
// failed key was absent from the "fetched" set, so the deletion pass read it as a
// record that no longer existed upstream and deleted a live config record from the
// edge — the write failure escalated into data loss.
func TestSyncCollectionFailedPutDoesNotPurgeTheKey(t *testing.T) {
	ctx := context.Background()

	// Both records are already mirrored and unchanged upstream.
	kv := newFakeKV(nil)
	pb := &fakeLister{records: []pbclient.Record{rec("id1", "alpha"), rec("id2", "beta")}}
	cache := newSyncCache()

	// Cycle 1: populate the mirror.
	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("first cycle: %v", err)
	}
	if got := kv.storedKeys(); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("after first cycle got keys %v, want [alpha beta]", got)
	}

	// Cycle 2: "alpha" changes upstream, but its Put fails.
	changed := rec("id1", "alpha")
	changed["name"] = "renamed"
	pb.records = []pbclient.Record{changed, rec("id2", "beta")}
	kv.failPut["alpha"] = true

	n, err := syncCollection(ctx, pb, kv, cache, "things")
	if err == nil {
		t.Fatal("expected the failed write to be surfaced as an error, got nil")
	}
	if n != 2 {
		t.Errorf("desired count = %d, want 2", n)
	}

	// The whole point: the key survives.
	if got := kv.storedKeys(); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Errorf("got keys %v, want [alpha beta] — a failed Put must never purge", got)
	}
	if len(kv.deletes) != 0 {
		t.Errorf("Delete called for %v; nothing was removed upstream", kv.deletes)
	}

	// And the retry happens next cycle, because the cache was not poisoned.
	kv.failPut = map[string]bool{}
	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("third cycle: %v", err)
	}
	if got := string(kv.store["alpha"]); !strings.Contains(got, "renamed") {
		t.Errorf("alpha was not retried after the failure; value = %s", got)
	}
}

// A key that disappears from KV out-of-band must be rewritten on the next cycle.
// Before the fix the cache alone decided whether to write, so once the daemon had
// written a key it never wrote it again — a bucket wiped underneath a long-running
// leaf-sync stayed empty until the process restarted.
func TestSyncCollectionRepairsOutOfBandKeyLoss(t *testing.T) {
	ctx := context.Background()

	kv := newFakeKV(nil)
	pb := &fakeLister{records: []pbclient.Record{rec("id1", "alpha"), rec("id2", "beta")}}
	cache := newSyncCache()

	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("first cycle: %v", err)
	}

	// Something else removes a key: `nats kv del`, a purged/recreated bucket, a
	// lost file store. PocketBase still reports the record, unchanged.
	delete(kv.store, "alpha")
	kv.puts = nil

	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("second cycle: %v", err)
	}

	if _, ok := kv.store["alpha"]; !ok {
		t.Error("alpha was not restored — a cache hit must be corroborated by the bucket")
	}
	if !reflect.DeepEqual(kv.puts, []string{"alpha"}) {
		t.Errorf("Put calls = %v, want exactly [alpha]: only the missing key should be rewritten", kv.puts)
	}
}

// --- surrounding behaviour these two fixes must not regress ------------------

func TestSyncCollectionSkipsUnchangedRecords(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(nil)
	pb := &fakeLister{records: []pbclient.Record{rec("id1", "alpha")}}
	cache := newSyncCache()

	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("first cycle: %v", err)
	}
	kv.puts = nil

	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("second cycle: %v", err)
	}
	if len(kv.puts) != 0 {
		t.Errorf("unchanged record was re-Put: %v", kv.puts)
	}
}

func TestSyncCollectionPurgesDeletedRecords(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(nil)
	pb := &fakeLister{records: []pbclient.Record{rec("id1", "alpha"), rec("id2", "beta")}}
	cache := newSyncCache()

	if _, err := syncCollection(ctx, pb, kv, cache, "things"); err != nil {
		t.Fatalf("first cycle: %v", err)
	}

	// "beta" is deleted upstream.
	pb.records = []pbclient.Record{rec("id1", "alpha")}
	n, err := syncCollection(ctx, pb, kv, cache, "things")
	if err != nil {
		t.Fatalf("second cycle: %v", err)
	}
	if n != 1 {
		t.Errorf("desired count = %d, want 1", n)
	}
	if got := kv.storedKeys(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Errorf("got keys %v, want [alpha]", got)
	}
	if !reflect.DeepEqual(kv.deletes, []string{"beta"}) {
		t.Errorf("deletes = %v, want [beta]", kv.deletes)
	}
	// The cache entry must go too, so a record that later reuses the key is written.
	if _, ok := cache.seen["things"]["beta"]; ok {
		t.Error("cache still holds the deleted key")
	}
}

func TestSyncCollectionEmptyFetchDoesNotPurge(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(map[string][]byte{"alpha": []byte(`{"id":"id1"}`)})
	pb := &fakeLister{records: nil}

	n, err := syncCollection(ctx, pb, kv, newSyncCache(), "things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
	if got := kv.storedKeys(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Errorf("got keys %v, want [alpha]: an empty fetch must never purge", got)
	}
	if len(kv.deletes) != 0 {
		t.Errorf("Delete called for %v on an empty fetch", kv.deletes)
	}
}

// An empty fetch against an empty bucket is the legitimate "nothing here" case and
// must not be mistaken for the guard above.
func TestSyncCollectionEmptyFetchEmptyBucketIsClean(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(nil)

	n, err := syncCollection(ctx, &fakeLister{}, kv, newSyncCache(), "things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}

func TestSyncCollectionListErrorLeavesKVAlone(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(map[string][]byte{"alpha": []byte(`{"id":"id1"}`)})
	pb := &fakeLister{err: errors.New("502 bad gateway")}

	if _, err := syncCollection(ctx, pb, kv, newSyncCache(), "things"); err == nil {
		t.Fatal("expected the list error to propagate")
	}
	if got := kv.storedKeys(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Errorf("got keys %v, want [alpha]: a fetch error must leave KV as-is", got)
	}
	if len(kv.deletes) != 0 || len(kv.puts) != 0 {
		t.Errorf("KV was written on a fetch error: puts=%v deletes=%v", kv.puts, kv.deletes)
	}
}

// A Keys failure means we cannot tell what the bucket holds, so we must not guess:
// no writes, no deletes, retry next cycle.
func TestSyncCollectionKeysErrorAborts(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(nil)
	kv.keysErr = errors.New("stream unavailable")
	pb := &fakeLister{records: []pbclient.Record{rec("id1", "alpha")}}

	if _, err := syncCollection(ctx, pb, kv, newSyncCache(), "things"); err == nil {
		t.Fatal("expected the Keys error to propagate")
	}
	if len(kv.puts) != 0 || len(kv.deletes) != 0 {
		t.Errorf("KV was written despite unknown state: puts=%v deletes=%v", kv.puts, kv.deletes)
	}
	if pb.calls != 0 {
		t.Errorf("PocketBase was queried %d times before the bucket state was known", pb.calls)
	}
}

// Two records sharing a code fall back to their ids, so neither shadows the other.
func TestSyncCollectionDuplicateCodesFallBackToID(t *testing.T) {
	ctx := context.Background()
	kv := newFakeKV(nil)
	pb := &fakeLister{records: []pbclient.Record{rec("id1", "dup"), rec("id2", "dup")}}

	n, err := syncCollection(ctx, pb, kv, newSyncCache(), "things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	if got := kv.storedKeys(); !reflect.DeepEqual(got, []string{"id1", "id2"}) {
		t.Errorf("got keys %v, want [id1 id2]", got)
	}
}
