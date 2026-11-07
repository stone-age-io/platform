package hooks

import (
	"errors"
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// ErrInfrastructureOrgActive is returned when something tries to clear `active`
// on the operator or system organization. Exported for errors.Is in tests.
var ErrInfrastructureOrgActive = errors.New("the platform's own organizations cannot be deactivated")

// OrgActiveFlagOptions names the two collections this mirror spans.
type OrgActiveFlagOptions struct {
	OrgCollection         string
	NatsAccountCollection string
}

// RegisterOrgActiveFlag makes `active` on organizations mean exactly one thing:
// the organization's NATS account is withdrawn. Nothing more. A flag nobody
// enforces is worse than no flag, because someone trusts it during an incident.
//
// `nats_accounts.active` is pb-nats's account-scope suspend switch: the
// true->false edge schedules a DELETE of the account claim, which disconnects
// every client of the tenant on the hub at once, and ReconcileAccounts skips
// inactive accounts, so it stays withdrawn across restarts.
//
// It is reversible, unlike the device flag in hooks/active_flag.go, whose
// revocation cutoff is permanent. This deletes the ACCOUNT claim and writes no
// cutoff, touches no nats_users row and re-mints nothing, so every creds_file
// works again on reactivation. That asymmetry is why a tenant-wide suspend is
// safe behind a checkbox and a tenant-wide revoke would not be.
//
// What it deliberately does not do:
//
//   - Touch Nebula. Revocation lands only on each peer's redeploy, and undoing
//     it rewrites every config in the mesh: the wrong shape for a reversible
//     suspension.
//   - Lock the console. That needs `organization.active = true` on eight read
//     rules plus a hook for the org switcher, since a rule cannot traverse
//     @request.body.current_organization. A suspended tenant still signs in and
//     reads what it owns. That is a product decision.
//   - Kill Things' PocketBase sessions. That needs the flag in things.authRule
//     plus a RefreshTokenKey() sweep, because an authRule is only checked at
//     the auth endpoint.
//   - Reach a site's leaf node. A leaf runs `resolver: MEMORY` with a preloaded
//     account JWT, which /api/me/leaf-config serves whether or not the account
//     is active. Devices on the leaf keep working; the gateway's uplink is in
//     the withdrawn account, so the site is islanded. Say "disconnects from the
//     hub", never "disconnects everything".
//
// The mirror is level-triggered, unlike hooks/active_flag.go. A reported
// failure is retried by saving again, which an edge trigger would ignore. And
// Record.Original() is the state at the last database READ, not before this
// write, so a record saved twice in memory shows no edge on the second save: an
// edge build mirrored a deactivation and then did not undo it. The compare
// costs one indexed lookup and also pulls back an account flipped directly in
// the PocketBase dashboard.
//
// The error is returned, not logged, also unlike hooks/active_flag.go: pb-nats
// does the NATS work on a queue, so what fails here is the Save, and swallowing
// it would leave the badge saying suspended while the tenant is live. As in
// hooks/org_provisioning.go, the work runs before e.Next() and the error is
// returned after it, so later handlers still run.
func RegisterOrgActiveFlag(app *pocketbase.PocketBase, opts OrgActiveFlagOptions) {
	// Refuse the flip on the platform's own organizations. OnRecordUpdate is the
	// last layer that can still say no. It is an invariant, not a permission, so
	// it is not in schema.json: a rule cannot say "this value, on these two rows".
	// The operator org's account is the hub every managed tenant's helpdesk.>
	// import resolves against; the system org holds the adopted $SYS records. An
	// operator who means to withdraw the hub clears `active` on the nats_accounts
	// row instead.
	//
	// Edge-keyed, unlike the mirror: a level check would refuse every save of an
	// infrastructure org that is already inactive, including the save that fixes
	// it. The Original() limitation (above) is accepted because only the record
	// API writes organizations, and it loads the row first.
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

	// The mirror. Registered in main.go AFTER RegisterOrgProvisioning: handlers
	// run in registration order and provisioning works before e.Next(), so a
	// missing account exists by the time this runs.
	app.OnRecordAfterUpdateSuccess(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		mirrorErr := mirrorOrgActive(e.App, opts, e.Record)

		if err := e.Next(); err != nil {
			return err
		}
		return mirrorErr
	})

	// Deleting an organization needs it suspended and empty first. A delete
	// blanks the relations into it rather than removing rows (the tenancy
	// sentinel note in AUTHORIZATION.md), so a live tenant would leave an active
	// NATS account with a blank organization, and inventory makes the delete fail
	// part-way under relation tenancy. Suspension is checked on the account, not
	// the flag, so a failed mirror cannot pass. Nebula hosts must be gone too,
	// because suspension does not touch Nebula. Returning before e.Next() is
	// deliberate: the delete must not proceed.
	app.OnRecordDelete(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		if err := deletableOrg(e.App, opts, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
}

// ErrOrgNotDeletable is returned when an organization delete is refused.
var ErrOrgNotDeletable = errors.New("organization cannot be deleted yet")

func deletableOrg(app core.App, opts OrgActiveFlagOptions, org *core.Record) error {
	name := org.GetString("name")
	if org.GetBool("is_operator_org") || org.GetBool("is_system_org") {
		return fmt.Errorf("%w: %q backs the platform itself", ErrOrgNotDeletable, name)
	}
	if opts.NatsAccountCollection != "" {
		account, err := findOrgOwned(app, opts.NatsAccountCollection, org.Id)
		if err != nil {
			return fmt.Errorf("look up the NATS account for organization %q: %w", name, err)
		}
		if account != nil && account.GetBool("active") {
			return fmt.Errorf("%w: suspend %q first (clear active), which withdraws its NATS account", ErrOrgNotDeletable, name)
		}
	}
	for _, col := range []string{thingsCollection, locationsCollection, "nebula_hosts"} {
		n, err := app.CountRecords(col, dbx.HashExp{"organization": org.Id})
		if err != nil {
			return fmt.Errorf("count %s in organization %q: %w", col, name, err)
		}
		if n > 0 {
			return fmt.Errorf("%w: %q still has %d %s; delete or move them first", ErrOrgNotDeletable, name, n, col)
		}
	}
	return nil
}

// mirrorOrgActive brings the organization's NATS account into line with the
// organization's own `active` flag, and reports what it could not do.
func mirrorOrgActive(app core.App, opts OrgActiveFlagOptions, org *core.Record) error {
	if opts.NatsAccountCollection == "" {
		return nil // not configured on this deployment
	}

	// Neither infrastructure org is mirrored. The $SYS account is adopted by
	// bootstrap, not provisioned (ensureOrgNatsAccount skips it too). The
	// operator org is skipped because a database predating this hook can already
	// hold it with `active` clear, and the refusal above only governs
	// transitions: the first unrelated save after an upgrade would otherwise
	// withdraw the hub account every managed tenant imports through.
	if org.GetBool("is_system_org") || org.GetBool("is_operator_org") {
		return nil
	}

	account, err := findOrgOwned(app, opts.NatsAccountCollection, org.Id)
	if err != nil {
		return fmt.Errorf("look up the NATS account for organization %q: %w", org.GetString("name"), err)
	}
	if account == nil {
		// Provisioning ran first in this chain and reported its own failure.
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
