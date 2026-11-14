package hooks

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// OrgProvisioningOptions captures what new organizations get provisioned with.
// main.go loads it from config.
type OrgProvisioningOptions struct {
	OrgCollection         string
	NatsAccountCollection string
	NebulaCACollection    string

	NatsMaxConnections   int
	NatsMaxSubscriptions int
	NatsMaxPayload       int

	// JetStream ceilings, in bytes. Disk tracks the account's plan. Memory is a
	// blast-radius limit, since memory-backed streams compete for RAM every
	// tenant needs: keep it small and the same for every plan. -1 is unlimited;
	// 0 disables JetStream for the account, digital twin included.
	NatsMaxJetStreamDiskStorage   int64
	NatsMaxJetStreamMemoryStorage int64

	NebulaDefaultCAValidityYears int
}

// Validate refuses account defaults that would provision a tenant that cannot
// use NATS. In nats-server.conf 0 means "server default"; in the account JWT
// these values are copied into, 0 means NONE (hooks/nats_limits.go), and
// provisioning is not a request, so nothing else refuses it.
//
// The JetStream ceilings are not checked: 0 there is a real setting.
func (o OrgProvisioningOptions) Validate() error {
	var zero []string
	for _, l := range []struct {
		key   string
		value int
	}{
		{"max_connections", o.NatsMaxConnections},
		{"max_subscriptions", o.NatsMaxSubscriptions},
		{"max_payload", o.NatsMaxPayload},
	} {
		if l.value == 0 {
			zero = append(zero, "nats.default_limits."+l.key)
		}
	}
	if len(zero) == 0 {
		return nil
	}
	return fmt.Errorf("%s set to 0. In a NATS account JWT, 0 means none, not unlimited, "+
		"so no client in a new organization could use NATS. Use -1 for unlimited",
		strings.Join(zero, ", "))
}

// RegisterOrgProvisioning gives every organization the NATS account and Nebula
// CA its tenants cannot function without (except the System org, which
// bootstrap adopts the pre-seeded $SYS records into instead).
//
// Failures are returned, not logged: otherwise an org whose NATS account could
// not be written reports success, and the first symptom is a device that
// cannot connect. The error cannot roll the organization back, because
// *AfterCreateSuccess runs after the commit (core/db.go, txInfo.OnComplete),
// but it reaches the caller: Save() returns it outside a transaction, and
// RunInTransaction joins it in one.
//
// The work runs before e.Next() and the error is returned after it. Returning
// early would skip every later handler (RegisterManagedOrgExports,
// RegisterOrgMembership), trading a reported NATS failure for an owner with no
// membership row.
//
// Also bound to update, so re-saving the organization retries a failed
// provision. It is create-if-missing, so a no-op save costs two indexed
// lookups. hooks/managed_org_exports.go has the same shape.
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
// The two halves are separate subsystems, so both are attempted and their
// errors joined: the first failure must not hide the second.
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
		return fmt.Errorf("NATS account collection %q not found: %w",
			opts.NatsAccountCollection, err)
	}

	orgName := org.GetString("name")
	rec := core.NewRecord(col)
	rec.Set("name", orgName)
	rec.Set("organization", org.Id)
	// Inherited from the organization, not hardcoded true: the mirror in
	// hooks/org_active_flag.go binds to update only, so this is what keeps an
	// org created inactive from getting a live account.
	rec.Set("active", org.GetBool("active"))
	rec.Set("max_connections", opts.NatsMaxConnections)
	rec.Set("max_subscriptions", opts.NatsMaxSubscriptions)
	// max_data stays unlimited: it is a CUMULATIVE byte cap, not a monthly
	// allowance, which has to be measured from $SYS. nats-server v2.15 does not
	// enforce it anyway (hooks/nats_limits.go).
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
		return fmt.Errorf("Nebula CA collection %q not found: %w",
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
// A genuine database error is NOT reported as "not found": a transient failure
// would otherwise create a second NATS account, a second signing identity
// nothing reconciles.
func findOrgOwned(app core.App, collection, orgID string) (*core.Record, error) {
	return findFirst(app, collection, "organization = {:org}", dbx.Params{"org": orgID})
}
