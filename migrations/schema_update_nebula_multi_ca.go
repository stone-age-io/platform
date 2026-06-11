package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_nebula_multi_ca re-imports schema.json so existing deployments
// pick up pb-nebula's multi-CA uniqueness model (pb-nebula only applies these
// indexes to fresh databases):
//   - nebula_networks: global-unique cidr_range → unique per CA
//     (ca_id+name, ca_id+cidr_range), so different orgs' meshes can reuse
//     the same CIDR or network name
//   - nebula_hosts: global-unique hostname → unique per network
//     (network_id+hostname)
//
// Safe on existing data: the old constraints were strictly tighter, so no
// rows can violate the new ones. Additive import (deleteMissing=false).
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
