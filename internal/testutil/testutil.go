// Package testutil provides the shared PocketBase test harness: a real app
// against a throwaway data dir, with the embedded libraries set up, the full
// migration set applied, and the platform's own provisioning hooks bound.
//
// The hooks matter: most of what a write does here happens in one, so a harness
// without them would assert on records production never produces.
package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	pbnats "github.com/skeeeon/pb-nats"
	pbnebula "github.com/skeeeon/pb-nebula"

	"platform/hooks"
	"platform/migrations"
)

// repoRoot resolves the module root from this file's own location, so the schema
// can be read regardless of which package's test is running.
func repoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot resolve caller path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", ".."), nil
}

// SkipIfShort skips a test that needs the real PocketBase harness when `-short`
// is set.
//
// In this repo `-short` skips only the tests that stand up a real PocketBase
// (~10s each, about thirty times a run). The real-nats-server tests in
// internal/health and internal/natsd are deliberately not skipped: they are
// cheap and assert the server's own trust decisions.
//
// SetupApp calls it, so new tests land in the right set. The two packages whose
// TestMain builds one shared app guard it themselves, since TestMain runs
// before any test can skip.
func SkipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("needs a real PocketBase (~10s to boot); run without -short")
	}
}

// SetupApp boots a real PocketBase against t.TempDir(), applies every migration,
// and binds the platform's provisioning hooks. The returned app is ready for
// record CRUD. Skipped under `-short` (see SkipIfShort).
//
// No NATS server is involved: pb-nats signs JWTs locally and the claim publish
// only queues.
func SetupApp(t *testing.T) *pocketbase.PocketBase {
	t.Helper()
	SkipIfShort(t)
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	return app
}

// NewApp is SetupApp without the testing.T, for a TestMain that shares one app
// across a package's read-only tests, paying the boot cost once.
//
// The caller owns the directory and must call ResetBootstrapState when done.
func NewApp(dataDir string) (*pocketbase.PocketBase, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}

	// migrations/initial_schema.go silently imports nothing when this is empty,
	// leaving an app with no platform fields and no error.
	schema, err := os.ReadFile(filepath.Join(root, "schema.json"))
	if err != nil {
		return nil, fmt.Errorf("read schema.json: %w", err)
	}
	migrations.SchemaJSON = schema

	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  dataDir,
		HideStartBanner: true,
	})

	// Mirrors main.go's viper defaults for the absorbed tenancy collections.
	const (
		orgCollection        = "organizations"
		membershipCollection = "memberships"
		inviteCollection     = "invites"
	)

	natsOpts := pbnats.DefaultOptions()
	nebulaOpts := pbnebula.DefaultOptions()

	// Push pb-nats's two publish timers (the 3s debounce and the 30s queue
	// ticker) past the life of any test. Neither is tied to the app's
	// lifecycle, so one firing after ResetBootstrapState panics with a nil
	// dereference in core.BaseApp.RecordQuery. No test exercises the publish
	// path; there is no NATS server.
	natsOpts.DebounceInterval = time.Hour
	natsOpts.PublishQueueInterval = time.Hour

	// First, as in main.go: a cross-tenant relation is refused before any
	// library acts on the record.
	hooks.RegisterRelationTenancy(app)

	if err := pbnats.Setup(app, natsOpts); err != nil {
		return nil, fmt.Errorf("nats setup: %w", err)
	}
	if err := pbnebula.Setup(app, nebulaOpts); err != nil {
		return nil, fmt.Errorf("nebula setup: %w", err)
	}

	hooks.RegisterOrgCode(app, orgCollection)
	hooks.RegisterCodes(app)
	hooks.RegisterLocationPath(app)
	// Changes what a write stores: a role saved without its limits gets -1, not
	// the 0 a JWT reads as "none". Before provisioning, as in main.go.
	hooks.RegisterNatsLimits(app, hooks.NatsLimitsOptions{
		RoleCollection:    natsOpts.RoleCollectionName,
		AccountCollection: natsOpts.AccountCollectionName,
	})
	hooks.RegisterOrgProvisioning(app, hooks.OrgProvisioningOptions{
		OrgCollection:                 orgCollection,
		NatsAccountCollection:         natsOpts.AccountCollectionName,
		NebulaCACollection:            nebulaOpts.CACollectionName,
		NatsMaxConnections:            100,
		NatsMaxSubscriptions:          5000,
		NatsMaxPayload:                1048576,
		NatsMaxJetStreamDiskStorage:   5 * 1024 * 1024 * 1024,
		NatsMaxJetStreamMemoryStorage: 64 * 1024 * 1024,
		NebulaDefaultCAValidityYears:  5,
	})
	// After provisioning, as in main.go: both handlers sit on the organization's
	// AfterUpdateSuccess and run in registration order, and this one assumes
	// the account already exists.
	hooks.RegisterOrgActiveFlag(app, hooks.OrgActiveFlagOptions{
		OrgCollection:         orgCollection,
		NatsAccountCollection: natsOpts.AccountCollectionName,
	})
	// Bound before app.Bootstrap(), as in main.go. A late bind now works (the
	// org hooks call e.Next()), but the harness is only equivalent to main.go
	// if it binds where main.go binds. Quiet for tests that do not care: it
	// acts only when organizations.managed is true.
	hooks.RegisterManagedOrgExports(app, hooks.ManagedOrgExportsOptions{
		OrgCollection:     orgCollection,
		AccountCollection: natsOpts.AccountCollectionName,
		ExportCollection:  natsOpts.ExportCollectionName,
		ImportCollection:  natsOpts.ImportCollectionName,
		ExportSubject:     "helpdesk.>", // main.go's default for nats.managed_export_subject
	})
	// Off, as main.go's default leaves it. A test that wants inventory events
	// binds a second, enabled registration (hooks/inventory_events_test.go).
	hooks.RegisterInventoryEvents(app, hooks.InventoryEventsOptions{
		Enabled:            false,
		OrgCollection:      orgCollection,
		AccountCollection:  natsOpts.AccountCollectionName,
		UserCollection:     natsOpts.UserCollectionName,
		RoleCollection:     natsOpts.RoleCollectionName,
		ExportCollection:   natsOpts.ExportCollectionName,
		ImportCollection:   natsOpts.ImportCollectionName,
		ThingCollection:    "things",
		LocationCollection: "locations",
		LocalSubject:       "stone-age.>",
	})

	// Gives a new organization's owner their `owner` membership row, which every
	// write rule resolves authority through. Bound where the absorbed pb-tenancy
	// hook used to run.
	hooks.RegisterOrgMembership(app, hooks.OrgMembershipOptions{
		OrgCollection:        orgCollection,
		MembershipCollection: membershipCollection,
		UserCollection:       "users",
	})
	// RegisterInvites binds record hooks and a route. Only the record hooks run
	// here (token, expiry and invited_by on create, reissue on resend); see the
	// boundary note at the bottom of this function.
	hooks.RegisterInvites(app, hooks.InviteOptions{
		MembershipCollection: membershipCollection,
		InviteCollection:     inviteCollection,
		UserCollection:       "users",
		ExpiryDays:           7, // main.go's default for tenancy.invite_expiry_days
	})

	// Both change what a write does. RegisterActiveFlag forces `active = true` on
	// every things create (bools have no schema default and the authRule needs
	// it); without it a test could create an inactive Thing production never
	// would. RegisterMembershipLifecycle clears a departing member's
	// current_organization, which the inventory read rules scope on.
	hooks.RegisterActiveFlag(app, hooks.ActiveFlagOptions{
		ThingCollection:      "things",
		NatsUserCollection:   natsOpts.UserCollectionName,
		NebulaHostCollection: nebulaOpts.HostCollectionName,
	})
	hooks.RegisterMembershipLifecycle(app, hooks.MembershipLifecycleOptions{
		MembershipCollection: membershipCollection,
		UserCollection:       "users",
	})

	// Deliberately not registered, because each binds only app.OnServe
	// (observability also OnTerminate) and this harness never serves:
	//
	//   RegisterLeafConfigRoutes, RegisterCredentialRoutes,
	//   RegisterNatsAccountRoutes, RegisterNebulaRoutes, RegisterThingRoutes,
	//   RegisterClientConfigRoutes, RegisterEmailTemplates, RegisterObservability
	//
	// RegisterActivity binds only the record REQUEST hooks, which record CRUD
	// never fires; hooks/activity_test.go binds it on its own app.
	//
	// Test routes with a served app or scripts/test-authz.sh. The rule: every
	// hook that changes what a write does is bound here; nothing that only
	// answers HTTP is.
	if err := app.Bootstrap(); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	runner := core.NewMigrationsRunner(app, core.AppMigrations)
	if _, err := runner.Up(); err != nil {
		return nil, fmt.Errorf("migrations up: %w", err)
	}
	return app, nil
}
