package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ActiveFlagOptions names the collections that carry an `active` flag whose
// meaning this file enforces.
type ActiveFlagOptions struct {
	ThingCollection    string
	LeafNodeCollection string
	NatsUserCollection string
}

// RegisterActiveFlag makes `active` on things and leaf_nodes mean something.
//
// The schema.json authRule on both collections is `active = true`, which stops a
// deactivated device from obtaining a NEW auth token. On its own that is close to
// worthless, for two reasons:
//
//  1. PocketBase evaluates authRule at the auth endpoint only — in
//     apis.RecordAuthResponse (apis/record_helpers.go), reached from
//     /auth-with-password and friends. It is NOT re-checked on requests that
//     carry an already-issued token. Both collections set authToken.duration to
//     604800, so a device deactivated at noon keeps its API access for a week.
//
//  2. A Thing's real capability is not its PocketBase session, it is the signed
//     NATS user JWT held by its linked nats_users record. Nothing PocketBase does
//     to the Thing touches that.
//
// So deactivation does three things, not one:
//
//	active = false  ->  authRule blocks new logins
//	                ->  RefreshTokenKey() invalidates every outstanding token now
//	                ->  revoke = true on the linked NATS identity
//
// and re-activation issues a fresh NATS credential, since the revocation cutoff
// in the account JWT is permanent and the old creds file stays dead.
//
// This exists because the codebase already had the other kind of `active`:
// nats_users.active is read into pb-nats's model (internal/types/converters.go)
// and then consulted by nothing in JWT generation or sync. Only `revoke` — which
// adds the public key to the account's revocation list and re-signs the account
// JWT (pb-nats internal/sync/manager.go, revokeUser) — actually disconnects
// anyone. A flag that turns a badge red while the device keeps publishing is
// worse than no flag, because someone will trust it during an incident.
//
// The NATS cascade is best-effort and logged, never fatal: matching
// RegisterLeafNodeProvisioning, a NATS hiccup must not roll back the operator's
// deactivation. The PocketBase half — the part that is transactional with the
// record write — always succeeds or fails with the save.
func RegisterActiveFlag(app *pocketbase.PocketBase, opts ActiveFlagOptions) {
	collections := []string{opts.ThingCollection, opts.LeafNodeCollection}

	// A device is born enabled. `active` is a bool, and PocketBase bools have no
	// schema-level default — an omitted field lands as false, which with the
	// authRule above would mean every API-created Thing is dead on arrival. It is
	// forced rather than defaulted-if-absent because the member branch of the
	// things create rule freezes `active`, so a member's create legitimately
	// carries no value, and "absent" and "explicitly false" are indistinguishable
	// by the time a hook sees the record. Deactivation is an update.
	app.OnRecordCreate(collections...).BindFunc(func(e *core.RecordEvent) error {
		e.Record.Set("active", true)
		return e.Next()
	})

	app.OnRecordUpdate(collections...).BindFunc(func(e *core.RecordEvent) error {
		was := e.Record.Original().GetBool("active")
		now := e.Record.GetBool("active")

		if was == now {
			return e.Next()
		}

		// Must happen before the save so the new tokenKey is written by it.
		// Changing tokenKey invalidates every auth token already issued for this
		// record — that is the whole point, and it is why deactivation cannot be
		// left to the authRule alone.
		if !now {
			e.Record.RefreshTokenKey()
		}

		if err := e.Next(); err != nil {
			return err
		}

		// Past this point the flip is committed. Cascade to NATS.
		natsUserID := e.Record.GetString("nats_user")
		if natsUserID == "" {
			return nil
		}

		natsUser, err := e.App.FindRecordById(opts.NatsUserCollection, natsUserID)
		if err != nil {
			log.Printf("⚠️ %s '%s' active=%v but its NATS identity %s could not be loaded: %v",
				e.Record.Collection().Name, e.Record.Id, now, natsUserID, err)
			return nil
		}

		if now {
			// Re-enable. `regenerate` mints a JWT with a later issue time, which
			// clears the account's revocation cutoff for this key; the previously
			// issued creds file stays rejected. Both flags are needed: pb-nats
			// sets active=false on revoke and does not set it back.
			natsUser.Set("active", true)
			natsUser.Set("regenerate", true)
		} else {
			// pb-nats watches `revoke` on OnRecordUpdate (the model hook, not the
			// request hook), so this app.Save is enough to trigger it.
			natsUser.Set("revoke", true)
		}

		if err := e.App.Save(natsUser); err != nil {
			log.Printf("❌ %s '%s' active=%v but the NATS cascade failed for identity %s: %v",
				e.Record.Collection().Name, e.Record.Id, now, natsUserID, err)
			return nil
		}

		if now {
			log.Printf("✅ %s '%s' reactivated; NATS identity %s re-issued",
				e.Record.Collection().Name, e.Record.Id, natsUserID)
		} else {
			log.Printf("🔒 %s '%s' deactivated; tokens invalidated and NATS identity %s revoked",
				e.Record.Collection().Name, e.Record.Id, natsUserID)
		}
		return nil
	})
}
