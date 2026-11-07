package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_nebula_multi_ca re-imports schema.json for pb-nebula's multi-CA
// uniqueness model, which pb-nebula applies only to fresh databases:
// nebula_networks become unique per CA (ca_id+name, ca_id+cidr_range), so orgs
// can reuse a CIDR or name, and nebula_hosts unique per network
// (network_id+hostname). The old constraints were tighter, so no row violates
// the new ones.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping nebula multi-CA index update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Nebula multi-CA indexes applied (networks unique per CA, hosts unique per network)")
		return nil
	}, nil)
}
