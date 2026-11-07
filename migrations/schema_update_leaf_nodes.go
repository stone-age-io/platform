package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_nodes adds the `leaf_nodes` auth collection (the edge sync
// identity) and read branches that let it mirror its own organization's config:
// the four inventory collections, its organization's nats_accounts row, and only
// its own linked nats_users row. nats_system_operator stays superuser-only; the
// operator JWT went through a route instead. Superseded by
// schema_update_drop_leaf_nodes.go.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping leaf_nodes schema update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ leaf_nodes collection + leaf-node read grants applied")
		return nil
	}, nil)
}
