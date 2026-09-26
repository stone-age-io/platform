// Package testutil provides the shared PocketBase test harness: a real app
// against a throwaway data dir, with the embedded libraries set up, the full
// migration set applied, and the platform's own provisioning hooks bound.
//
// The hooks are the point. Most of what this platform does on a write happens in
// a hook — an organization mints a NATS account and a Nebula CA, a deactivated
// Thing has its NATS identity revoked — so a harness that skipped them would let
// a test assert on records that production never produces the same way.
package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

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
// WHAT `-short` MEANS IN THIS REPO: skip the tests that stand up a real
// PocketBase. Nothing else. That is not a style choice, it is where the time
// actually is -- standing one app up costs the better part of ten seconds
// because four libraries bootstrap their collections and every migration runs,
// and `go test ./...` does it about thirty times. `hooks`, `internal/demoseed`
// and `migrations` are minutes; every other package is seconds.
//
// So the real-nats-server tests in internal/health and internal/natsd are
// deliberately NOT skipped. They cost about ten seconds between them, and they
// are the ones asserting the server's own trust decisions -- exactly the
// coverage you least want to drop from the pass you run most often. `-short`
// buys nothing by skipping them and costs the assertions a mock cannot make.
//
// This lives in SetupApp rather than in each test, so a test written next year
// is in the right set without anyone remembering. The two packages with a
// TestMain that builds ONE shared app guard it themselves; they have to, since
// TestMain runs before any test can be skipped.
func SkipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("needs a real PocketBase (~10s to boot); run without -short")
	}
}

// SetupApp boots a real PocketBase against t.TempDir(), applies every migration,
// and binds the platform's provisioning hooks. The returned app is ready for
// record CRUD.
//
// Skipped under `-short` -- see SkipIfShort.
//
// No NATS server is involved. pb-nats generates keys and signs JWTs locally; the
// claim publish queues and drains if a server ever appears, which in a test it
// never does.
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

// NewApp is SetupApp without the testing.T, for a TestMain that wants ONE app
// shared across a package's read-only tests. Standing an app up costs a few
// seconds — four libraries bootstrap their collections and every migration runs
// — so a package with a dozen assertions against the same fixture set pays that
// once rather than a dozen times.
//
// The caller owns the directory and must call ResetBootstrapState when done.
func NewApp(dataDir string) (*pocketbase.PocketBase, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}

	// migrations/initial_schema.go returns nil when this is empty — marking
	// itself applied while importing nothing — so a harness that forgot it would
	// produce an app with no platform fields and no error.
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

	// Push pb-nats's two publish timers past the life of any test.
	//
	// Saving a NATS account arms a 3s debounce (sync.Manager.enqueue) and a 30s
	// ticker (publisher.processQueuePeriodically), both of which call
	// ProcessPublishQueue — which runs a record query against the app. Neither is
	// tied to the app's lifecycle, so when a test finishes and ResetBootstrapState
	// drops the database handle, a timer that fires afterwards panics with a nil
	// dereference inside core.BaseApp.RecordQuery. It is a real ordering hazard in
	// the library rather than anything the caller does wrong; a long-lived server
	// never sees it because the app outlives every timer.
	//
	// Pushing both out is deliberate rather than incidental: nothing in the test
	// suite exercises the publish path, because there is no NATS server for a
	// claim to reach. What is under test is what lands in the DATABASE.
	natsOpts.DebounceInterval = time.Hour
	natsOpts.PublishQueueInterval = time.Hour

	// First, exactly as in main.go: a cross-tenant relation is refused before any
	// library acts on the record.
	hooks.RegisterRelationTenancy(app)

	if err := pbnats.Setup(app, natsOpts); err != nil {
		return nil, fmt.Errorf("nats setup: %w", err)
	}
	if err := pbnebula.Setup(app, nebulaOpts); err != nil {
		return nil, fmt.Errorf("nebula setup: %w", err)
	}

	hooks.RegisterOrgCode(app, orgCollection)
	hooks.RegisterCodes(app, membershipCollection)
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
	// After provisioning, exactly as main.go binds it: both handlers sit on the
	// organization's AfterUpdateSuccess and run in registration order, and the
	// mirror is written assuming the account already exists by the time it looks.
	// Binding it earlier here would make the harness disagree with the server on
	// the one ordering this hook depends on.
	hooks.RegisterOrgActiveFlag(app, hooks.OrgActiveFlagOptions{
		OrgCollection:         orgCollection,
		NatsAccountCollection: natsOpts.AccountCollectionName,
	})
	// Registered here rather than in the test that needs it, and it must still be
	// bound before app.Bootstrap() below.
	//
	// This used to be a hard requirement rather than a convention. pb-tenancy
	// deferred its own hook registration to OnBootstrap, and its organizations
	// AfterCreateSuccess handler returned without e.Next() -- so it TERMINATED
	// the chain, and every handler on that event bound after Bootstrap never ran,
	// silently. A test that called RegisterManagedOrgExports on the app this
	// function returns got no export, no import, and no error; a priority -9999
	// bind was the only thing that fired. That hook is now
	// hooks.RegisterOrgMembership and it calls e.Next(), so a late bind would
	// work. Registering before Bootstrap stays anyway: this harness is only
	// equivalent to main.go if it binds where main.go binds.
	//
	// Quiet for tests that do not care: syncManaged only provisions when
	// organizations.managed is true, and its removal path is silent when there
	// is nothing to remove.
	hooks.RegisterManagedOrgExports(app, hooks.ManagedOrgExportsOptions{
		OrgCollection:     orgCollection,
		AccountCollection: natsOpts.AccountCollectionName,
		ExportCollection:  natsOpts.ExportCollectionName,
		ImportCollection:  natsOpts.ImportCollectionName,
		ExportSubject:     "helpdesk.>", // main.go's default for nats.managed_export_subject
	})

	// Absorbed from pb-tenancy, and bound in the position the library's own
	// bootstrap-registered hook effectively occupied. RegisterOrgMembership is
	// what gives a new organization's owner their `owner` membership row, which
	// every write rule resolves authority through -- a harness without it would
	// produce owners who cannot write in their own tenant, which production
	// owners can.
	hooks.RegisterOrgMembership(app, hooks.OrgMembershipOptions{
		OrgCollection:        orgCollection,
		MembershipCollection: membershipCollection,
		UserCollection:       "users",
	})
	// RegisterInvites binds record hooks AND a route. Only the record hooks
	// matter here (this harness never serves), but they are the half that decides
	// what a write does: token, expiry and invited_by on create, reissue on
	// resend. See the note at the bottom of this function about that boundary.
	hooks.RegisterInvites(app, hooks.InviteOptions{
		MembershipCollection: membershipCollection,
		InviteCollection:     inviteCollection,
		UserCollection:       "users",
		ExpiryDays:           7, // main.go's default for tenancy.invite_expiry_days
	})

	// These two were missing, and their absence was not neutral -- it made the
	// harness DISAGREE with production about what a write does.
	//
	// RegisterActiveFlag forces `active = true` on every things CREATE,
	// because PocketBase bools have no schema default and the authRule is
	// `active = true`. Without it bound here, a test could create an inactive
	// device and assert on it happily while the real binary overwrote the flag --
	// which is exactly what happened: internal/demoseed asked for inactive Things
	// at create time, the harness let it, and `stone-age demo-seed` on a real
	// install produced none.
	//
	// RegisterMembershipLifecycle clears a departing member's
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

	// DELIBERATELY NOT REGISTERED, and this is the harness's actual boundary
	// rather than an oversight:
	//
	//   RegisterLeafConfigRoutes, RegisterCredentialRoutes,
	//   RegisterNatsAccountRoutes, RegisterThingRoutes,
	//   RegisterClientConfigRoutes, RegisterObservability
	//
	// Every one of those binds ONLY app.OnServe (observability also OnTerminate),
	// and this harness never serves -- it does record CRUD against a bootstrapped
	// app. Binding them would add no behaviour, so a test that needs to exercise
	// a route has to stand up a served app or use scripts/test-authz.sh, which is
	// what that script exists for. Observability would additionally start a
	// background prober that dials NATS, which no test has.
	//
	// So: every hook that changes what a WRITE does is bound here; nothing that
	// only answers HTTP is. Say which of those two a new registrar is before
	// deciding it belongs.
	if err := app.Bootstrap(); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	runner := core.NewMigrationsRunner(app, core.AppMigrations)
	if _, err := runner.Up(); err != nil {
		return nil, fmt.Errorf("migrations up: %w", err)
	}
	return app, nil
}
