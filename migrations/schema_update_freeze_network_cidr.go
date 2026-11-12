package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_freeze_network_cidr adds `@request.body.cidr_range:changed =
// false` to nebula_networks.updateRule.
//
// pb-nebula regenerates every host config when a network's CIDR changes and
// re-signs no certificate, and Nebula routes the overlay from the certificate's
// network. So a CIDR edit leaves every host in the network unable to reach its
// peers until each one is renewed by hand, and only a server log line says so.
// Recreating the network is the honest way to change it.
//
// An import only: a rule change, no new column, nothing to backfill. Like the
// other frozen fields, it binds the API and not a superuser in the admin panel.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping network CIDR freeze")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ nebula_networks.cidr_range is frozen after create")
		return nil
	}, nil)
}
