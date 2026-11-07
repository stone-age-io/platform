package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_node_manage_rule adds a `leaf_nodes` manageRule, mirroring
// its updateRule, so an owner or admin can reset a lost leaf-sync password from
// the console; without one only superusers could. leaf_nodes was later dropped
// (schema_update_drop_leaf_nodes.go).
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
