package migrations

import (
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_credential_scoping stops every organization member from reading
// every device credential in the organization, and gives the NATS and Nebula
// collections a role dimension.
//
// Any member, the kiosk role included, could create and update `nats_users`, and
// pb-nats copies `publish_permissions` verbatim into the JWT it signs. So a member
// could set their own identity to `publish: [">"]` and get a server-signed JWT
// for it. `nats_roles` allowed the same, and the Nebula collections let a member
// mint overlay identities and rewrite firewall rules.
//
//   - nats_users, nats_roles, nebula_hosts, nebula_networks: every rule now
//     requires owner or admin. nats_users reads keep the non-user branches and add
//     one for the identity linked to the caller's own membership, which the
//     browser's NATS connection needs (ui/src/stores/nats.ts).
//   - things create/update: the `nats_user` and `nebula_host` relations are
//     owner/admin only, since a Thing can read its linked identity's credential.
//   - things.deleteRule: owner/admin, like every other inventory collection.
//   - organizations.updateRule: operators only. The record carries the tenancy
//     flags and drives NATS and Nebula provisioning.
//
// creds_file and config_yaml are deliberately not `hidden`. The leak was a
// row-scoping problem, and the owning identity must read them through the API
// (the browser connection, the edge agent, the download button). Hiding them
// would need a download endpoint plus agent and UI changes to buy nothing.
//
// Self-service rotation is a route, POST /api/me/nats-creds/rotate
// (hooks/credential_routes.go), because a rule cannot allow `regenerate` and
// nothing else without a deny-list.
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

// reportOverbroadNatsPermissions lists NATS identities and roles that grant ">"
// on publish or subscribe, since any member could set those before this upgrade.
// It only reports: the system user legitimately holds ">", and narrowing
// automatically could cut off a working device.
func reportOverbroadNatsPermissions(app core.App) {
	type target struct{ collection, label string }
	for _, t := range []target{
		{"nats_users", "identity"},
		{"nats_roles", "role"},
	} {
		if _, err := app.FindCollectionByNameOrId(t.collection); err != nil {
			continue
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
// It scans the raw JSON for the token rather than unmarshalling, because the
// field is sometimes stored as a JSON string rather than an array.
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
