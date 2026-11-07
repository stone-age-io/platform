package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_bootstrap makes two rule changes.
//
//  1. It removes the leaf-node read branches from nats_users and nats_accounts.
//     Reading them through the CRUD API coupled the edge agent to the rules on
//     secret-bearing collections. A route returned the fixed list of public
//     trust material and the leaf's own credential instead (GET
//     /api/leaf/bootstrap, since replaced by GET /api/me/leaf-config). A leaf
//     node then read nothing in nats_* or nebula_*.
//  2. It closes a write hole on things and locations. The things branch
//     restricted fields without naming a role, and locations had no role check,
//     so `badge` (the kiosk role) could create and edit inventory. Both now
//     carry the owner/admin/member allowlist.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping leaf-bootstrap update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Leaf nodes no longer read nats_users/nats_accounts (see GET /api/leaf/bootstrap); badge excluded from inventory writes")

		warnIfEdgeAgentsPredateBootstrapRoute(app)

		return nil
	}, nil)
}

// warnIfEdgeAgentsPredateBootstrapRoute reminds the operator that existing leaf
// nodes may run a leaf-sync binary that still reads the removed branches. Only a
// re-run of `leaf-sync config` on such a binary fails, and it fails loudly.
func warnIfEdgeAgentsPredateBootstrapRoute(app core.App) {
	if _, err := app.FindCollectionByNameOrId("leaf_nodes"); err != nil {
		return
	}

	records, err := app.FindRecordsByFilter("leaf_nodes", "1=1", "created", 0, 0)
	if err != nil {
		log.Printf("⚠️ Could not review leaf_nodes: %v", err)
		return
	}
	if len(records) == 0 {
		return
	}

	log.Printf("🔎 %d leaf node(s) exist. Running `leaf-sync config` now requires an agent built after this upgrade — it reads GET /api/leaf/bootstrap instead of the nats_users/nats_accounts collections. Existing `leaf-sync run` daemons and already-generated nats-leaf.conf files are unaffected; update the binary before regenerating a leaf config.", len(records))
}
