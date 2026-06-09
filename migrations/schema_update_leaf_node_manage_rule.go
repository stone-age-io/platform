package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_node_manage_rule re-imports schema.json so existing
// deployments pick up the new `leaf_nodes` manageRule. With manageRule unset,
// only superusers could change a leaf node's PocketBase password — meaning a
// lost leaf-sync credential could not be recovered from the console. The rule
// now lets an org Admin/Owner reset their own organization's leaf node
// credentials (the UI's "Reset credentials" action), mirroring the collection's
// updateRule. Additive import (deleteMissing=false); safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping leaf_nodes manageRule update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ leaf_nodes manageRule applied (org Admins/Owners can reset leaf node credentials)")
		return nil
	}, nil)
}
