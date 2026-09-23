package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_read_indexes adds plain indexes for the reads the two big
// tables serve most, and drops one index that duplicated another.
//
// WHY THE EXISTING ONES DID NOT COUNT. things and locations already carried
// UNIQUE (organization, code) over non-blank codes, which looks like it covers
// an org-scoped read. It does not: SQLite uses a partial index only when the
// query itself states the partial condition, and no list rule does. So every
// Things list, every Locations tree and every expansion into either table was
// a full scan across every tenant. The partial indexes enforce uniqueness and
// nothing else.
//
// What is added, and the query each one is for:
//
//	things (organization)    every read rule and list view
//	things (location)        the things list on a location detail view, and
//	                         PocketBase's own reference lookup on location delete
//	locations (organization) same as things; LocationListView fetches the whole org
//	nats_users (role_id)     pb-nats re-signs every user of a role on each role save
//
// Deliberately NOT indexed: the per-organization collections (accounts, CAs,
// roles, networks, types) hold a handful of rows per tenant and a scan of them
// is free. nebula_hosts.network_id and nebula_networks.ca_id are already the
// leading column of their unique composites.
//
// DROPPED: memberships (user). UNIQUE (user, organization) leads with the same
// column, so it answers every query the single-column index did.
//
// A plain re-import applies all of this: indexes are collection properties,
// replaced wholesale on import, unlike field definitions (see CLAUDE.md on
// re-imports keeping the live field). TestReadIndexes checks the live tables
// rather than trusting that.
func init() {
	m.Register(AddReadIndexes, nil)
}

// AddReadIndexes is the migration body.
func AddReadIndexes(app core.App) error {
	if len(SchemaJSON) == 0 {
		log.Println("⚠️ SchemaJSON is empty, skipping read index update")
		return nil
	}

	if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
		return err
	}

	log.Println("✅ Read indexes applied to things, locations, nats_users; redundant memberships (user) dropped")
	return nil
}
