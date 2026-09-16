package migrations

import (
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Drops the leaf_nodes collection. An edge site is a Thing now.
//
// WHY. leaf_nodes was "a special thing": an auth collection holding a code, a
// JetStream domain, a list of collections to mirror, and one server-provisioned
// NATS user. Every one of those has gone somewhere else or gone away.
//
//   - The mirror is gone. `leaf-sync` copied an organization's config
//     collections into the edge's local KV so devices could read them offline.
//     Nothing consumed the mirrored rows -- no rule-router rule read them, no
//     firmware did -- so the whole mechanism was moving data nobody asked for.
//     `synced_collections` went with it.
//   - The domain is computed, not stored. A leaf node's JetStream domain is now
//     just its code (GET /api/me/leaf-config), so a column that could disagree
//     with the code no longer exists to disagree.
//   - The "is this an edge box" marker is thing_types. That relation already
//     existed; a second way to say the same thing is a second way to be wrong.
//   - The identity is the Thing. A gateway logs in as itself against `things`
//     and holds its NATS credential on its own nats_user relation, exactly like
//     any other device.
//
// What is left that a leaf node needed and a Thing did not is one route:
// GET /api/me/leaf-config (hooks/leaf_config_routes.go), which hands a Thing the
// public trust material its nats-server needs. The payload is public or
// self-owned, so serving it to any authenticated Thing is not a widening -- it
// is why no marker field was needed to gate it.
//
// WHY THE FOUR schema_update_leaf_*.go MIGRATIONS ABOVE ARE STILL HERE. Deleting
// an applied migration is not free: PocketBase records applied migrations by file
// name, and hooks/readiness.go's `schema_version` check FAILS when the database
// holds names this binary does not know -- so removing those files would put
// every existing deployment into a 503 that reads as "your pb_data is from a
// newer build". They stay, they are no-ops on a fresh database (the schema.json
// they re-import no longer carries leaf_nodes), and the log line one of them
// prints about creating the collection is immediately followed by this one
// dropping it.
//
// ORDER does not matter here, unlike DropContractLayer. Nothing relates INTO
// leaf_nodes -- its four relations all point outward -- so PocketBase has no
// reference to refuse the delete for, and none of the earlier migrations can
// recreate a collection that is no longer in the import.
func init() {
	m.Register(DropLeafNodes, nil)
}

// DropLeafNodes is the migration body. Exported and named for the same reason
// DropContractLayer is: a fresh database imports the already-thinned schema.json,
// so the migration set alone only ever exercises the no-op path and a green run
// would prove nothing about the removal.
func DropLeafNodes(app core.App) error {
	// The import first, so the collections that carried a `leaf_nodes` read
	// branch stop referring to it before it goes. Additive (deleteMissing=false),
	// which cannot remove a collection -- hence the explicit Delete below.
	if len(SchemaJSON) > 0 {
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
	}

	col, err := app.FindCollectionByNameOrId("leaf_nodes")
	if err != nil || col == nil {
		return nil // fresh database, or already dropped
	}

	// Read the identities off every leaf node BEFORE the rows go, so the
	// operator gets told about them. They are not touched -- see below.
	orphans, err := orphanedLeafIdentities(app)
	if err != nil {
		return err
	}

	if err := app.Delete(col); err != nil {
		return fmt.Errorf("delete leaf_nodes: %w", err)
	}
	log.Println("✅ Dropped the leaf_nodes collection")

	reportOrphanedLeafIdentities(orphans)
	return nil
}

// orphanedLeafIdentities lists the NATS users and Nebula hosts that each leaf
// node owned, before the leaf node rows are deleted.
type leafIdentities struct {
	natsUsers   []string
	nebulaHosts []string
}

func orphanedLeafIdentities(app core.App) (leafIdentities, error) {
	var out leafIdentities

	nodes, err := app.FindAllRecords("leaf_nodes")
	if err != nil {
		return out, nil // nothing readable; the delete below is still correct
	}
	for _, n := range nodes {
		label := n.GetString("code")
		if label == "" {
			label = n.Id
		}
		if id := n.GetString("nats_user"); id != "" {
			out.natsUsers = append(out.natsUsers, label+" -> "+id)
		}
		if id := n.GetString("nebula_host"); id != "" {
			out.nebulaHosts = append(out.nebulaHosts, label+" -> "+id)
		}
	}
	return out, nil
}

// reportOrphanedLeafIdentities tells the operator about credentials this
// migration deliberately LEFT WORKING.
//
// A leaf node's relations are all non-cascade, so deleting the collection leaves
// its nats_users and nebula_hosts rows behind, and both are signed, live
// credentials. Revoking them here would have been the tidy thing to do and the
// wrong one: those credentials are what the edge boxes at real sites are
// connected with right now, and a migration that runs on deploy is not a place to
// take a fleet off the bus. The platform's own rule applies -- deactivate, do not
// delete -- and deciding when is the operator's call, once each site is running
// an agent that has re-provisioned as a Thing.
func reportOrphanedLeafIdentities(o leafIdentities) {
	if len(o.natsUsers) == 0 && len(o.nebulaHosts) == 0 {
		return
	}
	log.Printf("🔎 leaf_nodes is gone, but the identities it owned are STILL LIVE and were left alone on purpose:")
	if len(o.natsUsers) > 0 {
		log.Printf("   nats_users (%d): %s", len(o.natsUsers), strings.Join(o.natsUsers, ", "))
	}
	if len(o.nebulaHosts) > 0 {
		log.Printf("   nebula_hosts (%d): %s", len(o.nebulaHosts), strings.Join(o.nebulaHosts, ", "))
	}
	log.Printf("   Each edge site is a Thing now. Re-provision its agent against the Thing, " +
		"confirm it connects, then deactivate the row above from the console -- that revokes the NATS " +
		"credential and blocklists the Nebula certificate. Deleting the row instead leaves the " +
		"certificate trusted until it expires, because revocation needs it to fingerprint.")
}
