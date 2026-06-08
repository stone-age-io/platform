package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_nodes re-imports the embedded schema.json so existing
// deployments pick up the new `leaf_nodes` auth collection and the read-grant
// rule additions that let a leaf node (the edge sync identity) mirror its own
// organization's config:
//
//   - new `leaf_nodes` auth collection (single nats_user relation, like `things`).
//   - things / locations / thing_types / location_types list+view: a leaf_nodes
//     identity may read records in its own organization (org-scoped, read-only).
//   - nats_accounts list+view: a leaf_nodes identity may read its own org's
//     account (for the account JWT).
//   - nats_users list+view: a leaf_nodes identity may read ONLY the NATS user
//     linked to it (via the leaf_nodes_via_nats_user back-relation).
//
// The operator JWT is deliberately NOT exposed via a collection rule;
// nats_system_operator stays superuser-only and the operator JWT is served by a
// dedicated, leaf-node-authenticated route (see hooks/leaf_node_routes.go).
//
// Additive import (deleteMissing=false); safe on fresh DBs.
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
