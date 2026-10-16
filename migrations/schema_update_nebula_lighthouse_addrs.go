package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_nebula_lighthouse_addrs re-imports schema.json so existing
// deployments pick up three new nebula_hosts columns:
//
//   - additional_host_ports — the rest of a lighthouse's static_host_map entry,
//     beyond public_host_port. Nebula's value there is a list, so a dual-stack
//     lighthouse, one on two internet paths, and a split-horizon deployment
//     are all one lighthouse with several endpoints.
//   - advertise_addrs — routable addresses a host REPORTS to the lighthouses.
//     The complement of static_host_map rather than a duplicate: behind cloud
//     1:1 NAT a host sees only its private address and would otherwise report
//     that one to every peer.
//   - local_allow_list — which of a host's own interface addresses it
//     advertises. Nebula's default is all of them, so a containerised edge
//     offers docker0 and every bridge to peers that then burn a handshake
//     timeout each.
//
// NO BACKFILL IS NEEDED, which is worth stating rather than leaving implied.
// All three are optional JSON columns, nothing in nebula_hosts' API rules
// reads them, and the generator omits each one when it is empty — so an
// existing host renders byte-identical config_yaml after this runs. That is
// the opposite of schema_update_device_active_flag.go, where a new column
// landed under a live authRule and every pre-existing row needed the UPDATE.
//
// pb-nebula would add these columns itself at bootstrap (addMissingFields adds
// any declared field a collection lacks), so this import is belt and braces
// rather than the only path. It is here because schema.json is this repo's
// record of what nebula_hosts holds and the console writes these fields —
// PocketBase silently drops a write to a field that does not exist, so the two
// should not be allowed to drift. The ids are PocketBase's deterministic form,
// "json" + crc32 of the field name, for the reason migrations/CLAUDE.md gives:
// a re-import keeps the LIVE field when the name matches and the id does not,
// so an id we invented would make this migration quietly inert on any
// deployment where pb-nebula got there first.
//
// Additive import (deleteMissing=false).
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping nebula lighthouse address update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Nebula host columns applied (additional_host_ports, advertise_addrs, local_allow_list)")
		return nil
	}, nil)
}
