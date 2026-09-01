// Package testutil provides the shared PocketBase test harness: a real app
// against a throwaway data dir, with the embedded libraries set up, the full
// migration set applied, and the platform's own provisioning hooks bound.
//
// The hooks are the point. Most of what this platform does on a write happens in
// a hook — an organization mints a NATS account and a Nebula CA, a leaf node
// mints its NATS user — so a harness that skipped them would let a test assert
// on records that production never produces the same way.
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
	pbtenancy "github.com/skeeeon/pb-tenancy"

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

// SetupApp boots a real PocketBase against t.TempDir(), applies every migration,
// and binds the platform's provisioning hooks. The returned app is ready for
// record CRUD.
//
// No NATS server is involved. pb-nats generates keys and signs JWTs locally; the
// claim publish queues and drains if a server ever appears, which in a test it
// never does.
func SetupApp(t *testing.T) *pocketbase.PocketBase {
	t.Helper()
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

	tenancyOpts := pbtenancy.DefaultOptions()
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

	if err := pbtenancy.Setup(app, tenancyOpts); err != nil {
		return nil, fmt.Errorf("tenancy setup: %w", err)
	}
	if err := pbnats.Setup(app, natsOpts); err != nil {
		return nil, fmt.Errorf("nats setup: %w", err)
	}
	if err := pbnebula.Setup(app, nebulaOpts); err != nil {
		return nil, fmt.Errorf("nebula setup: %w", err)
	}

	hooks.RegisterOrgCode(app, tenancyOpts.OrganizationsCollection)
	hooks.RegisterOrgProvisioning(app, hooks.OrgProvisioningOptions{
		OrgCollection:                tenancyOpts.OrganizationsCollection,
		NatsAccountCollection:        natsOpts.AccountCollectionName,
		NebulaCACollection:           nebulaOpts.CACollectionName,
		NatsMaxConnections:           10,
		NatsMaxSubscriptions:         50,
		NatsMaxPayload:               1048576,
		NebulaDefaultCAValidityYears: 5,
	})
	hooks.RegisterLeafNodeProvisioning(app, hooks.LeafNodeProvisioningOptions{
		LeafNodeCollection:    "leaf_nodes",
		NatsAccountCollection: natsOpts.AccountCollectionName,
		NatsUserCollection:    natsOpts.UserCollectionName,
		NatsRoleCollection:    natsOpts.RoleCollectionName,
	})

	if err := app.Bootstrap(); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	runner := core.NewMigrationsRunner(app, core.AppMigrations)
	if _, err := runner.Up(); err != nil {
		return nil, fmt.Errorf("migrations up: %w", err)
	}
	return app, nil
}
