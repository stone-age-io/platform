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
//	                ->  active = false on the linked NATS identity
//
// and re-activation issues a fresh NATS credential, since the revocation cutoff
// in the account JWT is permanent and the old creds file stays dead.
//
// The NATS half is a straight mirror of the flag, because `active` on nats_users
// is pb-nats's durable suspend switch (internal/sync/manager.go). It is
// edge-triggered: true->false revokes the user's public key on the owning account
// and deliberately issues nothing back, so the device holds no working credential
// until reactivated; false->true signs a JWT whose issue time is later than the
// revocation cutoff, which NATS accepts while the old creds file stays rejected.
//
// Do not reach for `revoke` here, despite the name. In pb-nats `revoke` is the
// "these credentials leaked" button: it rotates the key pair and immediately
// hands back a *working* replacement, leaving the user active. Using it to
// deactivate a device would re-credential the thing you just disabled. It is also
// checked before the active edge and returns early, so setting both flags in one
// save silently takes the revoke path — the failure mode is a deactivated Thing
// whose NATS identity is freshly re-issued and still publishing.
//
// A flag that turns a badge red while the device keeps publishing is worse than
// no flag, because someone will trust it during an incident.
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

		// Mirror the flag and nothing else. pb-nats watches `active` on
		// OnRecordUpdate (the model hook, not the request hook), so this one save
		// is what triggers the revoke or the reissue. Setting `regenerate` as well
		// would be worse than redundant: the active edge returns before the
		// regenerate branch runs, leaving the flag set on the row to fire on some
		// unrelated later save.
		natsUser.Set("active", now)

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
