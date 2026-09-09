package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ActiveFlagOptions names the collections that carry an `active` flag whose
// meaning this file enforces.
type ActiveFlagOptions struct {
	ThingCollection      string
	LeafNodeCollection   string
	NatsUserCollection   string
	NebulaHostCollection string
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
// So deactivation does four things, not one:
//
//	active = false  ->  authRule blocks new logins
//	                ->  RefreshTokenKey() invalidates every outstanding token now
//	                ->  active = false on the linked NATS identity
//	                ->  active = false on the linked Nebula host
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
// The Nebula half is the same mirror for the same reason, and it closed the last
// door left open: a device taken out of service kept a valid overlay-network
// certificate until it expired, so "decommissioned" closed the console door and
// the NATS door and left the mesh door wide open. `active` on nebula_hosts is
// what pb-nebula puts into every OTHER host's `pki.blocklist`, because Nebula
// has no CRL — revocation is a fingerprint list each peer carries, not a central
// record. Two consequences follow from that shape and neither is a bug here:
//
//   - It takes effect when each peer's config is redeployed and reloaded. The
//     platform's job ends when the material it hands out says the certificate is
//     refused, exactly as it ends at minting a NATS credential rather than
//     policing what connects with it.
//   - Revocation needs the certificate to still be in the database to fingerprint
//     it, so DEACTIVATING a device revokes it and DELETING one does not. That is
//     worth knowing before deleting a Thing you actually want off the mesh.
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

		// Past this point the flip is committed. Cascade to whichever identities
		// this device actually holds. The two are independent — a Thing may carry
		// a NATS identity, a Nebula host, both or neither — so neither is allowed
		// to short-circuit the other. The previous version returned early when
		// `nats_user` was empty, which would have skipped the Nebula half for any
		// device that had only a certificate.
		mirrorActiveFlag(e, opts.NatsUserCollection, "nats_user", "NATS identity", now)
		mirrorActiveFlag(e, opts.NebulaHostCollection, "nebula_host", "Nebula host", now)
		return nil
	})
}

// mirrorActiveFlag copies a device's `active` flag onto one linked identity.
//
// Mirror the flag and nothing else. Both libraries watch `active` on
// OnRecordUpdate — the model hook, not the request hook — so this one save is
// what triggers the revoke or the reissue. On the NATS side, setting
// `regenerate` as well would be worse than redundant: pb-nats checks the active
// edge first and returns before the regenerate branch runs, leaving the flag set
// on the row to fire on some unrelated later save.
//
// Best-effort and logged, never fatal, matching RegisterLeafNodeProvisioning: a
// hiccup in NATS or Nebula must not roll back the operator's deactivation. The
// part that is transactional with the record write — the token kill — has
// already succeeded by the time this runs.
func mirrorActiveFlag(e *core.RecordEvent, collection, relationField, label string, now bool) {
	if collection == "" {
		return // not configured on this deployment
	}
	linkedID := e.Record.GetString(relationField)
	if linkedID == "" {
		return // nothing linked
	}

	linked, err := e.App.FindRecordById(collection, linkedID)
	if err != nil {
		log.Printf("⚠️ %s '%s' active=%v but its %s %s could not be loaded: %v",
			e.Record.Collection().Name, e.Record.Id, now, label, linkedID, err)
		return
	}

	linked.Set("active", now)

	if err := e.App.Save(linked); err != nil {
		log.Printf("❌ %s '%s' active=%v but the cascade failed for %s %s: %v",
			e.Record.Collection().Name, e.Record.Id, now, label, linkedID, err)
		return
	}

	if now {
		log.Printf("✅ %s '%s' reactivated; %s %s re-enabled",
			e.Record.Collection().Name, e.Record.Id, label, linkedID)
	} else {
		log.Printf("🔒 %s '%s' deactivated; tokens invalidated and %s %s revoked",
			e.Record.Collection().Name, e.Record.Id, label, linkedID)
	}
}
