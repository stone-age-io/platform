package hooks

import (
	"errors"
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ErrInfrastructureOrgActive is returned when something tries to clear `active`
// on the operator or system organization. Exported so a test can errors.Is it
// rather than matching on the message.
var ErrInfrastructureOrgActive = errors.New("the platform's own organizations cannot be deactivated")

// OrgActiveFlagOptions names the two collections this mirror spans.
type OrgActiveFlagOptions struct {
	OrgCollection         string
	NatsAccountCollection string
}

// RegisterOrgActiveFlag makes `active` on organizations mean something.
//
// Until this existed it meant nothing at all. The field was in schema.json, the
// operator form carried a toggle for it, and the organization detail and list
// views badged it red or green -- and not one line of Go, TypeScript or rule
// text read it back. Deactivating a tenant stopped no login, disconnected no
// device, revoked no credential and withdrew no account. It is the same shape as
// the `nats_users.active` checkbox that was removed from the UI for the same
// reason: a flag nobody enforces is worse than no flag, because someone will
// trust it during an incident -- or, here, during a billing dispute.
//
// WHAT IT NOW MEANS, EXACTLY: the organization's NATS account is withdrawn.
// Nothing more. That is one sentence on purpose, and it is the sentence the
// console should print next to the badge.
//
// WHY THAT IS THE WHOLE FEATURE. `nats_accounts.active` is pb-nats's own
// account-scope suspend switch (internal/sync/manager.go, the account
// AfterUpdateSuccess hook): the true->false edge schedules a DELETE of the
// account claim, which disconnects every client in the tenant at once -- devices,
// edge agents and the console's own browser connection -- and ReconcileAccounts
// skips inactive accounts, so it stays withdrawn across restarts. There is no
// cheaper lever with the same reach, because the account is the boundary; and
// there is no need to fan out over things, nats_users or nebula_hosts to get it.
//
// WHY IT IS REVERSIBLE, WHICH THE USER-SCOPE FLAG IS NOT. Deactivating here
// deletes the ACCOUNT claim. It writes no revocation cutoff, touches no
// nats_users row and re-mints nothing, so every creds_file in the tenant stays
// valid and simply has nothing to connect to. Reactivation republishes the
// account and the same credentials work again. Contrast hooks/active_flag.go,
// where clearing `active` on a single device revokes its public key against a
// cutoff that is permanent, so coming back has to issue a NEW credential. That
// asymmetry is why a tenant-wide suspend is safe to put behind a checkbox and a
// tenant-wide revoke would not be.
//
// WHAT IT DELIBERATELY DOES NOT DO, so nobody has to re-derive it:
//
//   - It does not touch Nebula. Revocation there is a fingerprint in every OTHER
//     host's pki.blocklist, it lands only when each peer's config is redeployed,
//     and undoing it rewrites every config in the mesh. That is the wrong shape
//     for a reversible suspension, and it moves nothing at the moment of
//     suspension anyway.
//   - It does not lock the console. Reads are scoped on the active organization
//     with no role branch, so locking a tenant out means `organization.active =
//     true` on eight read rules plus a hook for the org switcher (a rule cannot
//     do it: @request.body.current_organization is a raw submitted string with
//     nothing to traverse into -- the hooks/relation_tenancy.go lesson). A
//     suspended tenant can still sign in and read what it owns. That is the
//     product decision, not an oversight.
//   - It does not kill Things' PocketBase sessions. With the account withdrawn a
//     device can read its own record and call /api/me/leaf-config, and nothing
//     else. Closing that would need `active = true` on the organization relation
//     in things.authRule PLUS a RefreshTokenKey() sweep over every Thing in the
//     org, because an authRule is a latch and those tokens live seven days.
//
// WHY LEVEL-TRIGGERED RATHER THAN EDGE-TRIGGERED, which is where this differs
// from hooks/active_flag.go. The mirror is reported rather than swallowed (see
// below), and a reported failure is only useful if there is something to do
// about it. On an edge trigger the retry is unavailable by construction: the
// second save sees was == now and returns early, so the one lever the operator
// has -- save it again -- does nothing. Comparing the two values and saving only
// on a difference costs one indexed lookup per organization save, which is the
// same trade hooks/org_provisioning.go already took to become retryable, and it
// picks up drift for free: an account whose flag was flipped directly in the
// PocketBase dashboard is pulled back into line by the next organization save.
// Because the compare happens before the save, an organization update that
// changes something else writes nothing here and pb-nats sees no event.
//
// There is also a harder reason, found by writing the edge version and running
// the tests against it. Record.originalData is refreshed in exactly one place --
// Record.PostScan, which runs when the row is READ from the database
// (core/record_model.go) -- and by nothing in the save path. So Original() means
// "the state at the last LOAD", not "the state before this write", and the two
// stop agreeing the moment one in-memory record is saved twice. Deactivate and
// then reactivate through the same record object and the second save presents no
// edge at all: Original() still reports the value from before the first one. The
// edge build of this hook mirrored the deactivation and then silently declined to
// undo it, which is a worse failure than the one this file was written to remove.
// hooks/active_flag.go can key on an edge because its trigger is a REST request,
// where PocketBase loads the row before applying the patch; a hook that must also
// hold for app.Save() cannot assume that.
//
// WHY THE ERROR IS RETURNED RATHER THAN LOGGED, which is also where this differs
// from hooks/active_flag.go. That file's cascade is best-effort because its
// failure mode is a NATS hiccup, and a bus outage must not roll back an operator
// deactivation. This one has a different failure mode: pb-nats does the NATS work
// on a queue, so what can fail here is the PocketBase Save -- a database or
// validation problem, not a network one. Swallowing it leaves the badge saying
// suspended while the tenant is live, which is the precise lie this file exists
// to remove. So it follows hooks/org_provisioning.go: the work happens before
// e.Next() and the error is returned after it, because returning early would
// silently delete every handler bound later in the chain.
func RegisterOrgActiveFlag(app *pocketbase.PocketBase, opts OrgActiveFlagOptions) {
	// Refuse the flip on the platform's own organizations.
	//
	// This is a validation and so it binds to OnRecordUpdate, the only layer that
	// can still say no -- AfterUpdateSuccess runs after the commit and could only
	// report. It is an invariant rather than a permission (the caller is already
	// an operator and organizations.updateRule said so), which is what keeps it
	// out of schema.json: a rule cannot express "this value, on these two rows".
	//
	// The operator organization is not a customer. Its account is the hub every
	// managed tenant's helpdesk.> import resolves against, so withdrawing it
	// takes out event delivery for every managed org on the platform -- a blast
	// radius wildly out of proportion to an unlabelled checkbox on one form. The
	// system organization holds the adopted $SYS records, which bootstrap seeds
	// and provisioning skips entirely.
	//
	// Neither is a lever we are removing, only a click we are refusing: an
	// operator who genuinely means to withdraw the hub can clear `active` on the
	// nats_accounts row, which is the record that says so.
	//
	// This one IS keyed on the transition, unlike the mirror below, and it
	// carries that choice's limitation knowingly: Original() is the state at the
	// last database read (see the note above), so a caller re-saving one
	// in-memory organization record twice could slip a flip past it. Nothing
	// writes organizations except the record API, which loads the row first, and
	// a level check here would refuse every save of an infrastructure org that is
	// already inactive -- including the save that would put it right. Keyed on
	// the transition, being already inactive costs nothing and is recoverable.
	app.OnRecordUpdate(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		orig := e.Record.Original()
		if orig == nil || orig.GetBool("active") == e.Record.GetBool("active") {
			return e.Next()
		}
		if e.Record.GetBool("is_operator_org") || e.Record.GetBool("is_system_org") {
			return fmt.Errorf("%w: %q backs the platform itself; to withdraw its NATS account, clear `active` on the account record",
				ErrInfrastructureOrgActive, e.Record.GetString("name"))
		}
		return e.Next()
	})

	// The mirror itself.
	//
	// Registered in main.go AFTER RegisterOrgProvisioning, which matters: both
	// bind here, handlers run in registration order, and provisioning does its
	// work before calling e.Next(). So an organization whose account was missing
	// has one by the time this runs, rather than being mirrored against nothing
	// and then handed a fresh account with the wrong flag.
	app.OnRecordAfterUpdateSuccess(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		mirrorErr := mirrorOrgActive(e.App, opts, e.Record)

		if err := e.Next(); err != nil {
			return err
		}
		return mirrorErr
	})
}

// mirrorOrgActive brings the organization's NATS account into line with the
// organization's own `active` flag, and reports what it could not do.
func mirrorOrgActive(app core.App, opts OrgActiveFlagOptions, org *core.Record) error {
	if opts.NatsAccountCollection == "" {
		return nil // not configured on this deployment
	}

	// Neither infrastructure organization is mirrored, and the second half of
	// that is not redundant with the refusal above.
	//
	// The System org's $SYS account is seeded by `superuser upsert` and adopted
	// by bootstrap rather than provisioned, so there is nothing here to mirror --
	// the same reason ensureOrgNatsAccount skips it.
	//
	// The operator org is skipped because the refusal above only governs
	// transitions from here on. A database that predates this file can already
	// hold an operator org with `active` clear, set through a toggle that
	// enforced nothing at the time; without this line the first unrelated save
	// of that record after an upgrade would withdraw the hub account every
	// managed tenant imports through. A deploy must not be able to do that.
	if org.GetBool("is_system_org") || org.GetBool("is_operator_org") {
		return nil
	}

	account, err := findOrgOwned(app, opts.NatsAccountCollection, org.Id)
	if err != nil {
		return fmt.Errorf("look up the NATS account for organization %q: %w", org.GetString("name"), err)
	}
	if account == nil {
		// Provisioning ran first in this same chain and reported its own failure
		// if it could not create one. Saying so twice would name the same problem
		// with the wrong noun.
		return nil
	}

	want := org.GetBool("active")
	if account.GetBool("active") == want {
		return nil
	}

	account.Set("active", want)
	if err := app.Save(account); err != nil {
		return fmt.Errorf("organization %q is active=%v but its NATS account could not be updated to match: %w",
			org.GetString("name"), want, err)
	}

	if want {
		log.Printf("✅ Organization '%s' reactivated; NATS account republished", org.GetString("name"))
	} else {
		log.Printf("🔒 Organization '%s' deactivated; NATS account withdrawn, every client in the tenant disconnects",
			org.GetString("name"))
	}
	return nil
}
