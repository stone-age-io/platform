package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_bootstrap does two unrelated-looking things that were found
// in the same pass over the leaf/edge path.
//
// 1. REMOVES THE LEAF-NODE READ BRANCHES from nats_users and nats_accounts.
//
// `leaf-sync config` used to assemble a leaf server's config by reading those two
// collections through the CRUD API, which required granting a leaf-node identity a
// read branch on each. Two problems with that. It coupled the edge agent to the
// shape of rules on secret-bearing collections — a correct tightening elsewhere
// could break every edge box's bootstrap, and had to be reasoned about every time
// those rules changed. And it stated the edge's blast radius indirectly, as
// "whatever those branches happen to match", rather than as a fixed list.
//
// Both values now come from GET /api/leaf/bootstrap
// (hooks/leaf_node_routes.go), which reads the records with the app's own
// privileges and returns six named fields: domain, code, creds, account_jwt,
// account_pub, operator_jwt. Everything there is either public trust material
// (the operator and account JWTs are validated by every server in the network) or
// the leaf's own credential, which it must hold in order to connect. Account
// seeds and signing keys were never reachable and still are not.
//
// After this migration a leaf-node identity can read: its own leaf_nodes record,
// and the allowlisted config collections it mirrors (things, locations,
// thing_types, location_types, thing_type_operations, message_schemas). Nothing
// in nats_* or nebula_* at all. That is finally the same statement the leaf-sync
// README makes.
//
// 2. CLOSES A BADGE WRITE HOLE ON things AND locations.
//
// `things` create/update admitted a member through a branch that constrained the
// FIELDS (nats_user and nebula_host unchanged) without naming a ROLE, and
// `locations` create/update carried no role check whatsoever. Both were therefore
// satisfied by `badge` — the kiosk role, the most restricted one, which has no
// inventory authority in the documented model and gets no inventory navigation in
// the UI. A badge holder could create and edit Things and Locations, and every
// edit propagates into each edge node's local KV mirror.
//
// This is the same failure as the `role ?!= "member"` deny-list removed earlier in
// this hardening pass: both described who was excluded, or what was untouchable,
// instead of stating which roles may act. The branches now carry the canonical
// row-correlated allowlist (owner, admin, member), so adding a fifth role cannot
// silently grant it inventory writes.
//
// Not changed: members keep create and edit on both collections, and the
// owner/admin-only split on the nats_user / nebula_host relations stands.
//
// Additive import (deleteMissing=false); safe on fresh DBs.
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

// warnIfEdgeAgentsPredateBootstrapRoute reminds the operator that leaf nodes
// provisioned before this upgrade are running a leaf-sync binary that may still
// reach for the collection reads this migration just removed.
//
// GET /api/leaf/operator-jwt is deliberately kept, so an old agent's `run` loop
// and its already-written nats-leaf.conf are unaffected. Only a re-run of
// `leaf-sync config` on an outdated binary would fail, and it fails loudly with a
// 404 rather than producing a broken config.
func warnIfEdgeAgentsPredateBootstrapRoute(app core.App) {
	if _, err := app.FindCollectionByNameOrId("leaf_nodes"); err != nil {
		return // collection not present on this deployment
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
