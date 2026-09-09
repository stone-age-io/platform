package leafsync

import (
	"context"
	"testing"

	"platform/internal/leafsync/pbclient"
)

// pagedLister serves a scripted sequence of pages, so a walk can be made to
// come up short the way a live one does when the result set shifts underneath
// it. The existing fakeLister always claims TotalPages: 1, so multi-page
// reconcile was never exercised at all.
type pagedLister struct {
	pages      [][]pbclient.Record
	totalItems int
	calls      int
}

func (f *pagedLister) List(_ context.Context, _ string, page, perPage int, _ string) (*pbclient.ListResult, error) {
	f.calls++
	var items []pbclient.Record
	if page >= 1 && page <= len(f.pages) {
		items = f.pages[page-1]
	}
	return &pbclient.ListResult{
		Page:       page,
		PerPage:    perPage,
		TotalItems: f.totalItems,
		TotalPages: len(f.pages),
		Items:      items,
	}, nil
}

// A walk that returns fewer records than it was promised must not purge.
//
// Every record the walk failed to see is indistinguishable from a record that
// was deleted upstream, so the deletion pass would remove live config from the
// edge mirror. One record short of a 400-record collection quietly deletes one
// live row; there is no error, and the next cycle puts it back, so the only
// symptom is a device that briefly cannot resolve something.
func TestShortFetchDoesNotPurgeTheMirror(t *testing.T) {
	// Three records exist upstream, but page 2 comes back empty -- the shape of
	// a result set that shifted between requests.
	lister := &pagedLister{
		pages:      [][]pbclient.Record{{rec("r1", "r1"), rec("r2", "r2")}, {}},
		totalItems: 3,
	}
	kv := newFakeKV(map[string][]byte{
		"r1": []byte(`{"id":"r1"}`),
		"r2": []byte(`{"id":"r2"}`),
		"r3": []byte(`{"id":"r3"}`),
	})

	if _, err := syncCollection(context.Background(), lister, kv, newSyncCache(), "things"); err != nil {
		t.Fatalf("syncCollection: %v", err)
	}

	if _, ok := kv.store["r3"]; !ok {
		t.Error("r3 was purged from the mirror because the walk did not return it -- " +
			"a partial fetch was read as an upstream deletion")
	}
	if len(kv.store) != 3 {
		t.Errorf("mirror holds %d keys, want 3", len(kv.store))
	}
}

// ...and the records the short walk DID return are still written. Skipping the
// purge must not also stop delivery of the rows in hand.
func TestShortFetchStillUpsertsWhatItFetched(t *testing.T) {
	lister := &pagedLister{
		pages:      [][]pbclient.Record{{rec("r1", "r1")}, {}},
		totalItems: 2,
	}
	kv := newFakeKV(nil)

	if _, err := syncCollection(context.Background(), lister, kv, newSyncCache(), "things"); err != nil {
		t.Fatalf("syncCollection: %v", err)
	}

	if _, ok := kv.store["r1"]; !ok {
		t.Error("r1 was not written: the short-fetch guard skipped the upserts as well as the purge")
	}
}

// Pair the guard with the behaviour it must not break: a COMPLETE walk still
// purges. Without this, disabling the purge outright would pass the test above.
func TestCompleteMultiPageFetchStillPurges(t *testing.T) {
	lister := &pagedLister{
		pages:      [][]pbclient.Record{{rec("r1", "r1"), rec("r2", "r2")}, {rec("r3", "r3")}},
		totalItems: 3,
	}
	kv := newFakeKV(map[string][]byte{
		"r1":    []byte(`{"id":"r1"}`),
		"r2":    []byte(`{"id":"r2"}`),
		"r3":    []byte(`{"id":"r3"}`),
		"stale": []byte(`{"id":"stale"}`),
	})

	if _, err := syncCollection(context.Background(), lister, kv, newSyncCache(), "things"); err != nil {
		t.Fatalf("syncCollection: %v", err)
	}

	if _, ok := kv.store["stale"]; ok {
		t.Error("a key with no upstream record survived a complete walk; the purge is not running")
	}
	if lister.calls != 2 {
		t.Errorf("lister was called %d times, want 2 -- the walk did not page", lister.calls)
	}
	for _, k := range []string{"r1", "r2", "r3"} {
		if _, ok := kv.store[k]; !ok {
			t.Errorf("%s missing from the mirror after a complete walk", k)
		}
	}
}
