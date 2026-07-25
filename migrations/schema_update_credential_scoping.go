package migrations

import (
	"log"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_credential_scoping stops every organization member from being able
// to read every device credential in their org, and gives the NATS/Nebula
// infrastructure collections the role dimension they never had.
//
// THE WORSE HALF WAS THE WRITE SIDE. `nats_users` accepted create and update from
// any member of the organization — including `badge`, the kiosk role — and
// pb-nats copies `publish_permissions` from that record verbatim into the user JWT
// it signs (pb-nats internal/jwt/generator.go). So any member could PATCH their
// own linked identity to `publish: [">"]` and be handed a *server-signed* JWT
// granting it. Nothing surfaced that in the UI. `nats_roles` reached the same
// outcome through the permission template, and `nebula_hosts` /
// `nebula_networks` let a member mint overlay-network identities and rewrite any
// host's firewall rules and overlay IP.
//
// That `nats_roles.deleteRule` was already role-gated while create and update were
// not is what identifies this as drift rather than a design decision.
//
// SCHEMA CHANGES (in schema.json):
//
//   - nats_users, nats_roles, nebula_hosts, nebula_networks: list/view/create/
//     update/delete now require owner or admin of the record's organization,
//     using the same canonical allowlist text as the other 30 gated rules.
//   - nats_users read rules keep their non-user branches (a Thing, a leaf node,
//     or the identity itself reading its own row) and gain one for users: the
//     single identity linked to the caller's own membership. That branch is what
//     closes the credential broadcast — a member now sees exactly one row instead
//     of every identity in the org — and it is also load bearing, because the
//     browser opens its NATS connection with that credential
//     (ui/src/stores/nats.ts).
//   - things.createRule / updateRule: members keep inventory create/edit, but the
//     `nats_user` and `nebula_host` relations are owner/admin only. A Thing may
//     read the credential of its own linked identity, so a member able to
//     re-point those relations at a privileged identity and then authenticate as
//     the Thing would have a credential-theft path.
//   - things.deleteRule: owner/admin. `things` was the only collection left with
//     a member-level delete — locations, thing_types, location_types,
//     message_schemas, thing_type_operations, leaf_nodes, nats_roles and
//     nebula_networks all already gated it. Deleting a Thing orphans any identity
//     attached to it and leaf-sync propagates the deletion into every edge node's
//     local KV mirror.
//   - organizations.updateRule: platform operators only. The `owner` branch is
//     removed — an organization record carries the tenancy flags (managed,
//     is_operator_org, is_system_org) and drives NATS account and Nebula CA
//     provisioning, so it is not tenant-editable. Nothing regresses: the only view
//     that wrote this collection is admin/OrganizationFormView, whose route is
//     already operator-gated.
//
// DELIBERATELY NOT DONE: creds_file and config_yaml are NOT flagged hidden. The
// broadcast was a row-scoping problem, not a field-visibility one, and those
// fields have to stay readable to the identity that owns them — the browser's own
// NATS connection, `leaf-sync config` on an edge box, and the admin download
// button all read them through the API. Hiding them would have required a
// download endpoint plus changes in the edge agent and five UI call sites to buy
// nothing the row scoping does not already give.
//
// Self-service credential rotation moved to POST /api/me/nats-creds/rotate
// (hooks/credential_routes.go), because it needs a single-field allowlist —
// `regenerate` and nothing else — which a rule can only approximate with an
// `:isset = false` deny-list that silently opens up whenever a field is added.
//
// Additive import (deleteMissing=false); safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping credential-scoping update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Credential broadcast closed: nats_users scoped per-identity, NATS/Nebula infrastructure now owner/admin only, Thing identity links admin-only")

		reportOverbroadNatsPermissions(app)

		return nil
	}, nil)
}

// reportOverbroadNatsPermissions lists NATS identities and roles that grant
// publish or subscribe on ">" (everything). Until this upgrade any member could
// set those, so an upgrading admin needs to know which ones exist before assuming
// the escalation was never used.
//
// It only reports. Narrowing a permission set automatically would revoke a
// working device mid-flight, and the platform's own system user legitimately
// holds ">" — there is no safe way to tell the two apart from here.
func reportOverbroadNatsPermissions(app core.App) {
	type target struct{ collection, label string }
	for _, t := range []target{
		{"nats_users", "identity"},
		{"nats_roles", "role"},
	} {
		if _, err := app.FindCollectionByNameOrId(t.collection); err != nil {
			continue // collection not present on this deployment
		}

		records, err := app.FindRecordsByFilter(t.collection, "1=1", "created", 0, 0)
		if err != nil {
			log.Printf("⚠️ Could not review %s for over-broad permissions: %v", t.collection, err)
			continue
		}

		var flagged []string
		for _, r := range records {
			if grantsEverything(r, "publish_permissions") || grantsEverything(r, "subscribe_permissions") {
				name := r.GetString("nats_username")
				if name == "" {
					name = r.GetString("name")
				}
				flagged = append(flagged, name+" (id="+r.Id+")")
			}
		}

		if len(flagged) > 0 {
			log.Printf("🔎 %d NATS %s(s) grant '>' (all subjects) — confirm each is deliberate:", len(flagged), t.label)
			for _, f := range flagged {
				log.Printf("     • %s", f)
			}
			log.Printf("⚠️ Before this upgrade any organization member could set these, and the resulting JWT is signed by the org account. The platform's own system user legitimately holds '>'; anything else on this list should be justified or narrowed.")
		}
	}
}

// grantsEverything reports whether a permission field contains the ">" wildcard.
// The field is JSON (a string array), and PocketBase hands it back as raw JSON, so
// this checks for the token rather than unmarshalling into a typed slice — the
// field is also sometimes stored as a JSON string rather than an array.
func grantsEverything(r *core.Record, field string) bool {
	raw := r.GetString(field)
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(strings.Trim(raw, "[]"), ",") {
		if strings.Trim(strings.TrimSpace(part), `"`) == ">" {
			return true
		}
	}
	return false
}
