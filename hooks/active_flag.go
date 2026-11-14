package hooks

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
)

// ActiveFlagOptions names the collection that carries an `active` flag whose
// meaning this file enforces, plus the two identity collections it cascades to.
type ActiveFlagOptions struct {
	ThingCollection      string
	NatsUserCollection   string
	NebulaHostCollection string
}

// RegisterActiveFlag makes `active` on things mean something. Deactivation does
// four things, because any one alone is a half-measure:
//
//	active = false  ->  authRule blocks new logins
//	                ->  RefreshTokenKey() invalidates every outstanding token now
//	                ->  active = false on the linked NATS identity
//	                ->  active = false on the linked Nebula host
//
// The authRule alone is not enough. PocketBase checks it only at the auth
// endpoint, never on an already-issued token (valid for a week here), and a
// Thing's real capability is its NATS JWT and Nebula certificate, which nothing
// done to the Thing record touches.
//
// NATS: `active` on nats_users is pb-nats's durable suspend switch, and it is
// edge-triggered. true->false revokes the user's public key on the account and
// issues nothing back; false->true signs a fresh JWT past the revocation cutoff.
// The cutoff is permanent, so the old creds file stays dead. The hub enforces it
// at once; a leaf (`resolver: MEMORY`) only after its agent refreshes the
// account JWT (platform.leaf_refresh_interval, or cmd.leaf_refresh).
//
// Mirror `active`, never `revoke`. In pb-nats `revoke` is the "credentials
// leaked" button: it hands back a working replacement and leaves the user
// active. It is also checked first, so setting both in one save silently takes
// the revoke path.
//
// Nebula: `active` on nebula_hosts feeds every other host's `pki.blocklist`,
// because Nebula has no CRL. So it takes effect only when each peer's config is
// redeployed, and it needs the certificate still in the database to fingerprint
// it: deactivating a device revokes it, deleting one does not. So a Thing with a
// live identity cannot be deleted at all (guardThingDelete).
//
// Deactivation fails closed. The identities are revoked BEFORE the Thing is
// saved, and if either revoke fails the save is refused, so the console never
// shows a device as cut off while its credential still works. A revoke touches
// only the database (pb-nats syncs the account JWT to NATS after the commit), so
// a NATS outage does not block it; what can fail is a missing account or a
// signing error, and those need an operator anyway. The token kill rides on the
// Thing's own save.
//
// A refusal can be partial: an identity revoked before the other one failed
// stays revoked, and the Thing stays active. That is the safe side, and saving
// the Thing inactive again finishes the job.
//
// The revoke half is LEVEL-keyed: every save that leaves a Thing inactive
// revokes whatever it links, not only the save that flips the flag. Otherwise
// linking a live identity to a Thing that is already inactive would leave a
// working credential behind a device the console shows as cut off. An identity
// that is already inactive is not saved again, so this costs nothing on an
// ordinary edit. The token kill stays on the edge.
//
// Reactivation runs the other way round and stays best-effort and logged: the
// Thing must be active first (guardLinkedIdentity), and a failure there leaves
// the device off, which is the safe side.
func RegisterActiveFlag(app *pocketbase.PocketBase, opts ActiveFlagOptions) {
	// A device is born enabled. PocketBase bools have no schema default, so an
	// omitted `active` would be false and the authRule would lock out every new
	// Thing. Forced rather than defaulted: the member create rule freezes
	// `active`, and by now "absent" and "explicitly false" look the same.
	app.OnRecordCreate(opts.ThingCollection).BindFunc(func(e *core.RecordEvent) error {
		e.Record.Set("active", true)
		return e.Next()
	})

	app.OnRecordUpdate(opts.ThingCollection).BindFunc(func(e *core.RecordEvent) error {
		was := e.Record.Original().GetBool("active")
		now := e.Record.GetBool("active")

		// A Thing may hold either identity, both or neither, so neither mirror
		// may short-circuit the other.
		mirrors := []struct{ collection, field, label string }{
			{opts.NatsUserCollection, "nats_user", "NATS identity"},
			{opts.NebulaHostCollection, "nebula_host", "Nebula host"},
		}

		if now {
			if err := e.Next(); err != nil {
				return err
			}
			if was {
				return nil // an ordinary edit of an active Thing
			}
			for _, m := range mirrors {
				if err := mirrorActiveFlag(e, m.collection, m.field, m.label, true); err != nil {
					log.Printf("❌ %s '%s' reactivated but its %s was not re-enabled: %v",
						e.Record.Collection().Name, e.Record.Id, m.label, err)
				}
			}
			return nil
		}

		for _, m := range mirrors {
			if err := mirrorActiveFlag(e, m.collection, m.field, m.label, false); err != nil {
				log.Printf("❌ %s '%s' not saved: it is inactive and its %s could not be revoked: %v",
					e.Record.Collection().Name, e.Record.Id, m.label, err)
				return apis.NewBadRequestError(
					"Thing "+thingName(e.Record)+" was not saved: it is deactivated, and its "+
						m.label+" could not be revoked, so its credential would have kept "+
						"working. Any identity already revoked stays revoked. Try again, "+
						"and check the server log if it fails again.",
					nil,
				)
			}
		}

		// Before the save, so the save writes the new tokenKey. That is what
		// kills every token already issued. Only on the edge: an inactive
		// Thing holds no live token to kill.
		if was {
			e.Record.RefreshTokenKey()
		}
		return e.Next()
	})

	// The mirror above runs one way, so the identities need a guard in the
	// other: see suspendedByThing.
	guardLinkedIdentity(app, opts.ThingCollection, opts.NatsUserCollection, "nats_user",
		"NATS identity", []string{"regenerate", "revoke"})
	guardLinkedIdentity(app, opts.ThingCollection, opts.NebulaHostCollection, "nebula_host",
		"Nebula host", []string{"renew"})

	guardThingDelete(app, opts)
}

// guardThingDelete refuses to delete a Thing while an identity it links is
// still active. A delete reaches neither identity: the NATS credential keeps
// working, the Nebula certificate stays trusted, and once the Thing is gone
// nothing points an operator at either one. Deactivating first revokes both
// (the mirror above), and then the delete goes through.
//
// It checks the identities, not the Thing's own flag. The flag is not proof: a
// database written before the revoke failed closed, or one edited around the
// hooks, can hold a deactivated Thing with a live identity. And a Thing that
// never had one is a plain inventory row with nothing to revoke.
//
// A model hook, so it also holds for app.Delete and the admin UI. Negative
// priority, so it refuses before any other delete handler acts.
func guardThingDelete(app *pocketbase.PocketBase, opts ActiveFlagOptions) {
	linked := []struct{ collection, field, label string }{
		{opts.NatsUserCollection, "nats_user", "NATS identity"},
		{opts.NebulaHostCollection, "nebula_host", "Nebula host"},
	}

	app.OnRecordDelete(opts.ThingCollection).Bind(&hook.Handler[*core.RecordEvent]{
		Priority: -100,
		Func: func(e *core.RecordEvent) error {
			for _, l := range linked {
				id := e.Record.GetString(l.field)
				if l.collection == "" || id == "" {
					continue
				}
				identity, err := e.App.FindRecordById(l.collection, id)
				if errors.Is(err, sql.ErrNoRows) {
					continue // the relation points at nothing
				}
				if err != nil {
					// Fail closed: this is an enforcement hook.
					return fmt.Errorf("checking the %s of the Thing: %w", l.label, err)
				}
				if !identity.GetBool("active") {
					continue
				}

				return apis.NewBadRequestError(
					"Thing "+thingName(e.Record)+" still has an active "+l.label+". A delete would leave "+
						"its credential working. Deactivate the Thing first, then delete it.",
					nil,
				)
			}
			return e.Next()
		},
	})
}

// guardLinkedIdentity refuses to bring back an identity whose Thing is
// deactivated. The mirror runs one way; without this guard, the NATS user page's
// Re-enable, the Nebula form's Active toggle or a PATCH puts the device back on
// the bus or the mesh while the Thing says it is cut off. Reactivating the Thing
// is the way back.
//
// Refused while a Thing linking the identity is inactive:
//   - the identity being active after the save. LEVEL, not edge: Original() is
//     the last database read (hooks/CLAUDE.md). Saving it inactive, the repair,
//     still works.
//   - a minting trigger. pb-nats mints a working credential for an inactive user
//     on `regenerate` and `revoke`. pb-nebula refuses `renew` on an inactive host
//     since v0.3.3, but `renew` stays listed so the invariant does not rest on a
//     library version.
//
// The mirror itself always passes, because it runs after the Thing's own save.
// Negative priority so it runs before pb-nats and pb-nebula mint on the same
// save.
func guardLinkedIdentity(app *pocketbase.PocketBase, thingCollection, identityCollection, relationField, label string, triggers []string) {
	if identityCollection == "" {
		return // not configured on this deployment
	}

	app.OnRecordUpdate(identityCollection).Bind(&hook.Handler[*core.RecordEvent]{
		Priority: -100,
		Func: func(e *core.RecordEvent) error {
			reviving := e.Record.GetBool("active")
			for _, f := range triggers {
				if e.Record.GetBool(f) {
					reviving = true
				}
			}
			if !reviving {
				return e.Next()
			}

			thing, err := e.App.FindFirstRecordByFilter(thingCollection,
				relationField+" = {:id} && active = false", dbx.Params{"id": e.Record.Id})
			if errors.Is(err, sql.ErrNoRows) {
				return e.Next() // no deactivated Thing links it
			}
			if err != nil {
				// Fail closed: this is an enforcement hook.
				return fmt.Errorf("checking for a deactivated Thing: %w", err)
			}

			return apis.NewBadRequestError(
				"This "+label+" belongs to Thing "+thingName(thing)+", which is deactivated. "+
					"Reactivate the Thing to bring it back.",
				nil,
			)
		},
	})
}

// thingName is how an error message names a Thing: its code, or its name if
// it has none.
func thingName(thing *core.Record) string {
	if code := thing.GetString("code"); code != "" {
		return code
	}
	return thing.GetString("name")
}

// mirrorActiveFlag copies a device's `active` flag onto one linked identity and
// returns why it could not; the caller decides whether that refuses the save.
// A relation that points at nothing has nothing to revoke, so it is not an
// error.
//
// Only the flag. Both libraries watch `active` on the model hook, so this save
// triggers the revoke or reissue. Setting `regenerate` too would be worse than
// redundant: pb-nats handles the active edge first and returns, leaving
// `regenerate` set on the row to fire on a later, unrelated save.
func mirrorActiveFlag(e *core.RecordEvent, collection, relationField, label string, now bool) error {
	if collection == "" {
		return nil // not configured on this deployment
	}
	linkedID := e.Record.GetString(relationField)
	if linkedID == "" {
		return nil // nothing linked
	}

	linked, err := e.App.FindRecordById(collection, linkedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // the relation points at nothing
	}
	if err != nil {
		return fmt.Errorf("loading %s %s: %w", label, linkedID, err)
	}
	if linked.GetBool("active") == now {
		return nil // already there; a save would only churn the libraries
	}

	linked.Set("active", now)
	if err := e.App.Save(linked); err != nil {
		return fmt.Errorf("saving %s %s: %w", label, linkedID, err)
	}

	if now {
		log.Printf("✅ %s '%s' reactivated; %s %s re-enabled",
			e.Record.Collection().Name, e.Record.Id, label, linkedID)
	} else {
		log.Printf("🔒 %s '%s' deactivating; %s %s revoked",
			e.Record.Collection().Name, e.Record.Id, label, linkedID)
	}
	return nil
}
