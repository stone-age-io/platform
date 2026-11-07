package migrations_test

import (
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"platform/internal/testutil"
	"platform/migrations"
)

// removed names every field the drop migration removes, keyed by the id the
// migration uses. A lookup by name would pass against a rebuilt collection.
var removed = []struct {
	collection string
	field      string
	id         string
}{
	{"thing_type_operations", "schema", "relation_tto_schema"},
	{"thing_types", "capabilities", "select490417661"},
	{"thing_types", "nats_role", "relation_tt_nats_role"},
}

// restorePreDropSchema puts the old schema back through the real importer, so
// the state under test matches a live database. The fixture lists only the
// removed fields of the two partial collections; an additive import keeps every
// other live field.
func restorePreDropSchema(t *testing.T, app *pocketbase.PocketBase) {
	t.Helper()

	fixture, err := os.ReadFile(filepath.Join("testdata", "pre_drop_contract_layer.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := app.ImportCollectionsByMarshaledJSON(fixture, false); err != nil {
		t.Fatalf("restore pre-drop schema: %v", err)
	}

	if _, err := app.FindCollectionByNameOrId("message_schemas"); err != nil {
		t.Fatalf("fixture did not restore the message_schemas collection: %v", err)
	}
	for _, r := range removed {
		col, err := app.FindCollectionByNameOrId(r.collection)
		if err != nil {
			t.Fatalf("find %s: %v", r.collection, err)
		}
		if col.Fields.GetById(r.id) == nil {
			t.Fatalf("fixture did not restore %s.%s (id %s)", r.collection, r.field, r.id)
		}
		if !tableHasColumn(t, app, r.collection, r.field) {
			t.Fatalf("fixture restored the %s.%s definition but not its column", r.collection, r.field)
		}
	}
}

// TestDropContractLayerRemovesTheOldSchema is the test `migrate up` cannot
// provide: a fresh database has nothing to remove, so this restores the old
// shape first.
func TestDropContractLayerRemovesTheOldSchema(t *testing.T) {
	app := testutil.SetupApp(t)
	restorePreDropSchema(t, app)

	if err := migrations.DropContractLayer(app); err != nil {
		t.Fatalf("DropContractLayer: %v", err)
	}

	if col, err := app.FindCollectionByNameOrId("message_schemas"); err == nil && col != nil {
		t.Error("message_schemas collection survived the migration")
	}

	for _, r := range removed {
		col, err := app.FindCollectionByNameOrId(r.collection)
		if err != nil {
			t.Fatalf("find %s: %v", r.collection, err)
		}
		if col.Fields.GetById(r.id) != nil {
			t.Errorf("%s.%s definition survived the migration", r.collection, r.field)
		}
		if tableHasColumn(t, app, r.collection, r.field) {
			t.Errorf("%s.%s column survived the migration", r.collection, r.field)
		}
	}
}

// TestDropContractLayerIsIdempotent covers a fresh database and a re-run.
func TestDropContractLayerIsIdempotent(t *testing.T) {
	app := testutil.SetupApp(t)

	if err := migrations.DropContractLayer(app); err != nil {
		t.Fatalf("DropContractLayer on a thinned database: %v", err)
	}
	// And again, after a restore-then-drop cycle.
	restorePreDropSchema(t, app)
	if err := migrations.DropContractLayer(app); err != nil {
		t.Fatalf("DropContractLayer after restore: %v", err)
	}
	if err := migrations.DropContractLayer(app); err != nil {
		t.Fatalf("DropContractLayer second run: %v", err)
	}
}

// TestMessageSchemasCannotBeDeletedWhileItsRelationExists pins the migration's
// step order: PocketBase refuses to delete a collection with relation references
// to it. A reorder would fail only on real databases with the old schema.
func TestMessageSchemasCannotBeDeletedWhileItsRelationExists(t *testing.T) {
	app := testutil.SetupApp(t)
	restorePreDropSchema(t, app)

	col, err := app.FindCollectionByNameOrId("message_schemas")
	if err != nil {
		t.Fatalf("find message_schemas: %v", err)
	}

	if err := app.Delete(col); err == nil {
		t.Fatal("deleting message_schemas succeeded while thing_type_operations.schema " +
			"still referenced it; PocketBase's reference check has changed and the " +
			"migration's step order no longer needs to be what it is")
	}
}
