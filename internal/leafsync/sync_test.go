package leafsync

import (
	"reflect"
	"testing"

	"platform/internal/leafsync/pbclient"
)

func TestResolveCollections(t *testing.T) {
	cases := []struct {
		name string
		leaf pbclient.Record
		want []string
	}{
		{
			name: "allowlist filters secrets and dedupes",
			leaf: pbclient.Record{"synced_collections": []any{
				"things", "locations", "nats_users", "things", "edges", 42,
			}},
			want: []string{"things", "locations"},
		},
		{
			name: "all four allowed",
			leaf: pbclient.Record{"synced_collections": []any{
				"things", "locations", "thing_types", "location_types",
			}},
			want: []string{"things", "locations", "thing_types", "location_types"},
		},
		{
			name: "graph collections allowed",
			leaf: pbclient.Record{"synced_collections": []any{
				"thing_types", "thing_type_operations", "message_schemas",
			}},
			want: []string{"thing_types", "thing_type_operations", "message_schemas"},
		},
		{
			name: "missing field",
			leaf: pbclient.Record{},
			want: nil,
		},
		{
			name: "non-array value",
			leaf: pbclient.Record{"synced_collections": "things"},
			want: nil,
		},
		{
			name: "empty array",
			leaf: pbclient.Record{"synced_collections": []any{}},
			want: nil,
		},
		{
			name: "only disallowed",
			leaf: pbclient.Record{"synced_collections": []any{"nats_users", "nats_accounts", "nebula_hosts"}},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveCollections(c.leaf)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("resolveCollections = %v, want %v", got, c.want)
			}
		})
	}
}

// countCandidates mirrors the tally syncCollection builds before keying.
func countCandidates(recs []pbclient.Record) map[string]int {
	counts := make(map[string]int)
	for _, r := range recs {
		if c := candidateKey(r); c != "" {
			counts[c]++
		}
	}
	return counts
}

func TestRecordKey(t *testing.T) {
	cases := []struct {
		name    string
		records []pbclient.Record
		// want maps each record id to its expected key.
		want map[string]string
	}{
		{
			name:    "code wins when present, unique and valid",
			records: []pbclient.Record{{"id": "rec0000000000001", "code": "S01"}},
			want:    map[string]string{"rec0000000000001": "S01"},
		},
		{
			name:    "missing code falls back to id",
			records: []pbclient.Record{{"id": "rec0000000000002"}},
			want:    map[string]string{"rec0000000000002": "rec0000000000002"},
		},
		{
			name:    "empty code falls back to id",
			records: []pbclient.Record{{"id": "rec0000000000003", "code": ""}},
			want:    map[string]string{"rec0000000000003": "rec0000000000003"},
		},
		{
			name: "duplicate code falls back to id for all sharers",
			records: []pbclient.Record{
				{"id": "rec0000000000004", "code": "DUP"},
				{"id": "rec0000000000005", "code": "DUP"},
				{"id": "rec0000000000006", "code": "UNIQ"},
			},
			want: map[string]string{
				"rec0000000000004": "rec0000000000004",
				"rec0000000000005": "rec0000000000005",
				"rec0000000000006": "UNIQ",
			},
		},
		{
			name:    "code with spaces is not a valid KV key, falls back to id",
			records: []pbclient.Record{{"id": "rec0000000000007", "code": "has space"}},
			want:    map[string]string{"rec0000000000007": "rec0000000000007"},
		},
		{
			name:    "name is used when there is no code (e.g. thing_type_operations)",
			records: []pbclient.Record{{"id": "rec0000000000010", "name": "read_temp"}},
			want:    map[string]string{"rec0000000000010": "read_temp"},
		},
		{
			name:    "code wins over name when both present",
			records: []pbclient.Record{{"id": "rec0000000000011", "code": "S01", "name": "Front Sensor"}},
			want:    map[string]string{"rec0000000000011": "S01"},
		},
		{
			name: "duplicate name falls back to id",
			records: []pbclient.Record{
				{"id": "rec0000000000012", "name": "read_temp"},
				{"id": "rec0000000000013", "name": "read_temp"},
			},
			want: map[string]string{
				"rec0000000000012": "rec0000000000012",
				"rec0000000000013": "rec0000000000013",
			},
		},
		{
			name:    "code fenced by dot falls back to id",
			records: []pbclient.Record{{"id": "rec0000000000008", "code": ".lead"}},
			want:    map[string]string{"rec0000000000008": "rec0000000000008"},
		},
		{
			name:    "message_schema composite wins over code precedence",
			records: []pbclient.Record{{"id": "rec0000000000009", "namespace": "sensors", "name": "temp", "version": "1"}},
			want:    map[string]string{"rec0000000000009": "sensors__temp__1"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			counts := countCandidates(c.records)
			for _, rec := range c.records {
				id, _ := rec["id"].(string)
				if got := recordKey(rec, counts); got != c.want[id] {
					t.Errorf("recordKey(%v) = %q, want %q", rec, got, c.want[id])
				}
			}
		})
	}
}

func TestStrip(t *testing.T) {
	rec := pbclient.Record{
		"id":             "rec0000000000001",
		"code":           "S01",
		"collectionId":   "pbc_123",
		"collectionName": "things",
		"expand":         map[string]any{"type": "sensor"},
		"created":        "2026-01-01 00:00:00Z",
		"updated":        "2026-06-08 00:00:00Z",
	}
	got := strip(rec)
	for _, gone := range serverOnlyFields {
		if _, ok := got[gone]; ok {
			t.Errorf("strip kept server-only field %q", gone)
		}
	}
	for _, kept := range []string{"id", "code", "created", "updated"} {
		if _, ok := got[kept]; !ok {
			t.Errorf("strip dropped field %q (should be kept)", kept)
		}
	}
}

func TestKeysToDelete(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		fetched  map[string]bool
		want     []string
	}{
		{
			name:     "purges keys absent from fetch",
			existing: []string{"a", "b", "c"},
			fetched:  map[string]bool{"a": true, "c": true},
			want:     []string{"b"},
		},
		{
			name:     "nothing stale",
			existing: []string{"a", "b"},
			fetched:  map[string]bool{"a": true, "b": true},
			want:     nil,
		},
		{
			name:     "empty bucket",
			existing: nil,
			fetched:  map[string]bool{"a": true},
			want:     nil,
		},
		{
			name:     "all stale preserves order",
			existing: []string{"z", "y", "x"},
			fetched:  map[string]bool{},
			want:     []string{"z", "y", "x"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := keysToDelete(c.existing, c.fetched)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("keysToDelete = %v, want %v", got, c.want)
			}
		})
	}
}
