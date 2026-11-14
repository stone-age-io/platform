package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_read_indexes adds plain indexes for the reads the two big
// tables serve most, and drops one index that duplicated another.
//
// The partial UNIQUE (organization, code) indexes on things and locations do
// not serve org-scoped reads: SQLite uses a partial index only when the query
// states its condition, and no list rule does.
//
// What is added, and the query each one is for:
//
//	things (organization)    every read rule and list view
//	things (location)        the things list on a location detail view, and
//	                         PocketBase's own reference lookup on location delete
//	locations (organization) same as things; LocationListView fetches the whole org
//	nats_users (role_id)     pb-nats re-signs every user of a role on each role save
//
// Not indexed: the small per-organization collections, where a scan is free.
// nebula_hosts.network_id and nebula_networks.ca_id already lead their unique
// composites. Dropped: memberships (user), which UNIQUE (user, organization)
// covers.
//
// A re-import applies indexes, which are replaced wholesale, unlike field
// definitions (migrations/CLAUDE.md). TestReadIndexes checks the live tables.
func init() {
	m.Register(AddReadIndexes, nil)
}

// AddReadIndexes is the migration body.
func AddReadIndexes(app core.App) error {
	if len(SchemaJSON) == 0 {
		return errNoSchema
	}

	if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
		return err
	}

	log.Println("✅ Read indexes applied to things, locations, nats_users; redundant memberships (user) dropped")
	return nil
}
