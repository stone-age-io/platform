package migrations

import (
	"fmt"
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// Drops the leaf_nodes collection. An edge site is a Thing now: it logs in
// against `things`, its JetStream domain is its code, thing_types marks it as an
// edge box, and GET /api/me/leaf-config (hooks/leaf_config_routes.go) serves the
// trust material its nats-server needs. The `leaf-sync` KV mirror went too,
// since nothing read the mirrored rows.
//
// The schema_update_leaf_*.go migrations stay. PocketBase records applied
// migrations by file name, and the `schema_version` readiness check
// (hooks/readiness.go) fails when the database holds names the binary does not
// know, so deleting them would put every existing deployment into a 503.
//
// Order does not matter, unlike DropContractLayer: nothing relates into
// leaf_nodes.
func init() {
	m.Register(DropLeafNodes, nil)
}

// DropLeafNodes is the migration body, exported so a test can run it against
// the old shape: a fresh database never has leaf_nodes to drop.
func DropLeafNodes(app core.App) error {
	// Import first, so collections with a `leaf_nodes` read branch stop
	// referring to it. An additive import cannot remove a collection, hence the
	// Delete below.
	if len(SchemaJSON) > 0 {
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
	}

	col, err := app.FindCollectionByNameOrId("leaf_nodes")
	if err != nil || col == nil {
		return nil // fresh database, or already dropped
	}

	// Read the identities before the rows go, so the operator can be told.
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

// leafIdentities holds the NATS users and Nebula hosts the leaf nodes owned.
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
// migration deliberately left working. The relations are non-cascade, so the
// nats_users and nebula_hosts rows survive, and live edge boxes are connected
// with them. A deploy must not take a fleet off the bus; the operator
// deactivates each once its site has re-provisioned as a Thing.
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
