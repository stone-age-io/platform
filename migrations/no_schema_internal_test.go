package migrations

import (
	"errors"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// With SchemaJSON unset, `migrate up` must fail with errNoSchema. A guard that
// logged and returned nil let the migration be recorded as applied having
// imported nothing, so it never ran again once the wiring was fixed.
func TestMigrateUpFailsWithoutSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("boots a PocketBase app")
	}

	saved := SchemaJSON
	SchemaJSON = nil
	t.Cleanup(func() { SchemaJSON = saved })

	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })

	_, err := core.NewMigrationsRunner(app, core.AppMigrations).Up()
	if !errors.Is(err, errNoSchema) {
		t.Fatalf("migrate up with no schema: want errNoSchema, got %v", err)
	}
}
