package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_nebula_lighthouse_addrs re-imports schema.json for three new
// nebula_hosts columns:
//
//   - additional_host_ports: further static_host_map endpoints for a lighthouse
//     (dual-stack, two internet paths, split horizon).
//   - advertise_addrs: routable addresses a host reports to the lighthouses,
//     needed behind 1:1 NAT where it sees only its private address.
//   - local_allow_list: which of a host's own interface addresses it advertises.
//     Nebula's default is all, so a containerised edge offers docker0 too.
//
// No backfill: the columns are optional, no API rule reads them, and the
// generator omits them when empty, so existing configs render unchanged.
//
// pb-nebula also adds these columns at bootstrap; schema.json declares them so
// console writes are never silently dropped. The ids are PocketBase's
// deterministic "json" + crc32 of the name, because a re-import keeps the LIVE
// field when the name matches and the id does not (migrations/CLAUDE.md).
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Nebula host columns applied (additional_host_ports, advertise_addrs, local_allow_list)")
		return nil
	}, nil)
}
