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
