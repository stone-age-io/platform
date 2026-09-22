package hooks

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// OrgProvisioningOptions captures what new organizations get provisioned with.
// Kept as a plain struct so the caller (main.go) owns the config-loading story.
type OrgProvisioningOptions struct {
	OrgCollection         string
	NatsAccountCollection string
	NebulaCACollection    string

	NatsMaxConnections   int
	NatsMaxSubscriptions int
	NatsMaxPayload       int

	// JetStream ceilings, in bytes, and the two do different jobs. Disk tracks
	// what the account's plan is sold with. Memory is a blast-radius limit: a
	// memory-backed stream competes for the RAM every other tenant on the box
	// needs, so its breach is the one that is not confined to the account that
	// caused it -- keep it small and the same for every plan.
	//
	// -1 is unlimited. 0 disables JetStream for the account outright, which
	// takes the digital twin with it, so it is never the value you want here.
	NatsMaxJetStreamDiskStorage   int64
	NatsMaxJetStreamMemoryStorage int64

	NebulaDefaultCAValidityYears int
}

// RegisterOrgProvisioning gives every organization the NATS account and Nebula
// CA its tenants cannot function without (except the System org, which the
// bootstrap command adopts the pre-seeded $SYS records into instead).
//
// WHY THE FAILURES ARE RETURNED RATHER THAN LOGGED. They used to be neither:
// a missing collection was skipped by an `if err == nil` with no else, and a
// failed Save was a log.Printf. So creating an organization whose NATS account
// could not be written returned success, and what the operator got was a tenant
// that can never issue a device credential -- the org list shows it, the console
// shows it, and the first symptom is a device that cannot connect, later, for no
// stated reason. That is the same shape as the three unguarded client calls
// POST /api/org/things was written to replace, one level up: a provisioning step
// whose partial failure is invisible.
//
// An error here cannot roll the organization back -- PocketBase defers
// *AfterCreateSuccess to transaction completion (core/db.go, txInfo.OnComplete)
// and the row is committed by the time this runs. What it can do is reach the
// caller: outside a transaction core.BaseApp.create returns the hook's error
// straight from Save(), and inside one runAfterFuncs joins it into what
// RunInTransaction returns. Every caller surfaces that -- the record API as a
// failed response, bootstrap as a fatal, demoseed as a returned error. So the
// organization still exists and is still half-provisioned; the difference is
// that somebody is told.
//
// WHY THE WORK HAPPENS BEFORE e.Next() AND THE ERROR AFTER IT. Returning early
// would skip every handler bound after this one -- RegisterManagedOrgExports and
// RegisterOrgMembership -- because hook.Trigger chains handlers through
// e.Next(). That is the failure CLAUDE.md documents at length, and it would
// trade a reported NATS failure for an owner with no membership row, who cannot
// write anything in the tenant they just created. The provisioning runs where it
// always did (managed exports read the account this creates), the chain
// completes, and only the reporting is deferred.
//
// WHY IT IS ALSO BOUND TO UPDATE. An error is only useful if there is something
// to do about it, and until now the single trigger was create -- so a failure
// left no lever but deleting the organization and making it again. Provisioning
// is create-if-missing, so re-saving the organization retries it and costs two
// indexed lookups when there is nothing to do. This is the shape
// hooks/managed_org_exports.go already uses for the same reason, and the
// existence check is the same one hooks/org_membership.go grew.
func RegisterOrgProvisioning(app *pocketbase.PocketBase, opts OrgProvisioningOptions) {
	provision := func(e *core.RecordEvent) error {
		// The System org adopts the pre-seeded $SYS records in bootstrap instead.
		// Flag is authoritative; the name check covers DBs bootstrapped before it existed.
		if e.Record.GetBool("is_system_org") || e.Record.GetString("name") == "System" {
			return e.Next()
		}

		provisionErr := ensureOrgInfrastructure(e.App, opts, e.Record)

		if err := e.Next(); err != nil {
			return err
		}
		return provisionErr
	}

	app.OnRecordAfterCreateSuccess(opts.OrgCollection).BindFunc(provision)
	app.OnRecordAfterUpdateSuccess(opts.OrgCollection).BindFunc(provision)
}

// ensureOrgInfrastructure creates whichever of the organization's NATS account
// and Nebula CA is missing, and reports what it could not create.
//
// The two halves are attempted independently and their errors joined, for the
// reason hooks/active_flag.go gives for the mirrored cascade and bootstrap.go
// gives for its `problems` slice: they are separate subsystems, one being down
// says nothing about the other, and an operator fixing them one run at a time
// because the first failure hid the second is a worse morning than one that
// names both.
func ensureOrgInfrastructure(app core.App, opts OrgProvisioningOptions, org *core.Record) error {
	return errors.Join(
		ensureOrgNatsAccount(app, opts, org),
		ensureOrgNebulaCA(app, opts, org),
	)
}

func ensureOrgNatsAccount(app core.App, opts OrgProvisioningOptions, org *core.Record) error {
	if opts.NatsAccountCollection == "" {
		return nil // not configured on this deployment
	}

	existing, err := findOrgOwned(app, opts.NatsAccountCollection, org.Id)
	if err != nil {
		return fmt.Errorf("look up the NATS account for organization %q: %w", org.GetString("name"), err)
	}
	if existing != nil {
		return nil
	}

	col, err := app.FindCollectionByNameOrId(opts.NatsAccountCollection)
	if err != nil {
		// Previously an `if err == nil` with no else, so a renamed or missing
		// collection skipped provisioning in silence.
		return fmt.Errorf("NATS account collection %q not found (check nats.account_collection_name): %w",
			opts.NatsAccountCollection, err)
	}

	orgName := org.GetString("name")
	rec := core.NewRecord(col)
	rec.Set("name", orgName)
	rec.Set("organization", org.Id)
	// Inherited from the organization rather than hardcoded true, so the create
	// path agrees with hooks/org_active_flag.go. That mirror only binds to
	// update -- it has nothing to compare against on a create -- so an
	// organization created with `active` already clear would otherwise be handed
	// a live NATS account and stay that way until somebody saved it again.
	rec.Set("active", org.GetBool("active"))
	rec.Set("max_connections", opts.NatsMaxConnections)
	rec.Set("max_subscriptions", opts.NatsMaxSubscriptions)
	// max_data stays unlimited because it is the wrong shape for the thing the
	// price sheet promises: it is a CUMULATIVE byte cap in the account JWT, not
	// a monthly one, so it cannot express a bandwidth allowance. A per-month
	// figure has to be measured from $SYS, not fenced here.
	rec.Set("max_data", -1)
	rec.Set("max_payload", opts.NatsMaxPayload)
	rec.Set("max_jetstream_disk_storage", opts.NatsMaxJetStreamDiskStorage)
	rec.Set("max_jetstream_memory_storage", opts.NatsMaxJetStreamMemoryStorage)

	if err := app.Save(rec); err != nil {
		return fmt.Errorf("create the NATS account for organization %q: %w", orgName, err)
	}
	log.Printf("✅ Created NATS Account for '%s'", orgName)
	return nil
}

func ensureOrgNebulaCA(app core.App, opts OrgProvisioningOptions, org *core.Record) error {
	if opts.NebulaCACollection == "" {
		return nil // not configured on this deployment
	}

	existing, err := findOrgOwned(app, opts.NebulaCACollection, org.Id)
	if err != nil {
		return fmt.Errorf("look up the Nebula CA for organization %q: %w", org.GetString("name"), err)
	}
	if existing != nil {
		return nil
	}

	col, err := app.FindCollectionByNameOrId(opts.NebulaCACollection)
	if err != nil {
		return fmt.Errorf("Nebula CA collection %q not found (check nebula.ca_collection_name): %w",
			opts.NebulaCACollection, err)
	}

	orgName := org.GetString("name")
	rec := core.NewRecord(col)
	rec.Set("name", orgName+" CA")
	rec.Set("organization", org.Id)
	rec.Set("validity_years", opts.NebulaDefaultCAValidityYears)

	if err := app.Save(rec); err != nil {
		return fmt.Errorf("create the Nebula CA for organization %q: %w", orgName, err)
	}
	log.Printf("✅ Created Nebula CA for '%s'", orgName)
	return nil
}

// findOrgOwned returns the organization's record in `collection`, or nil when
// there is none.
//
// A genuine database error is NOT reported as "not found", which is the
// distinction internal/demoseed's ensure() spells out: treating every error as
// absence makes a transient failure create a duplicate of a record that already
// exists -- here, a second NATS account for an organization that already has
// one, which is a second signing identity nothing will ever reconcile.
func findOrgOwned(app core.App, collection, orgID string) (*core.Record, error) {
	rec, err := app.FindFirstRecordByFilter(collection, "organization = {:org}", dbx.Params{"org": orgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}
