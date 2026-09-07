package migrations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketbase/pocketbase"

	"platform/internal/testutil"
	"platform/migrations"
)

// removed names every field the drop migration is responsible for, with the id
// the migration keys on. Keyed by id because that is what the migration uses;
// a test that looked these up by name would pass against a collection that had
// been rebuilt with different ids and prove nothing.
var removed = []struct {
	collection string
	field      string
	id         string
}{
	{"thing_type_operations", "schema", "relation_tto_schema"},
	{"thing_types", "capabilities", "select490417661"},
	{"thing_types", "nats_role", "relation_tt_nats_role"},
}

// restorePreDropSchema puts the old schema back through the REAL importer, so
// the state under test is the state a live database is actually in rather than
// one assembled field by field in Go.
//
// testdata/pre_drop_contract_layer.json holds the message_schemas collection
// whole, plus partial entries for the two collections that lost fields. Partial
// is enough: ImportCollections with deleteMissing=false re-adds every live field
// the import did not carry by id, so listing only the removed fields restores
// exactly the pre-migration shape and leaves everything else alone.
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

// TestDropContractLayerRemovesTheOldSchema is the test the migration set cannot
// provide on its own. A fresh database imports the already-thinned schema.json,
// so `migrate up` reaches this migration with nothing left to remove and passes
// without touching anything. This restores the old shape first, so the removal
// is what is under test.
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

// TestDropContractLayerIsIdempotent covers the path a fresh database takes, and
// the path a re-run takes if the migration set is ever replayed.
func TestDropContractLayerIsIdempotent(t *testing.T) {
	app := testutil.SetupApp(t)

	// First call against an already-thinned database: nothing to remove.
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

// TestMessageSchemasCannotBeDeletedWhileItsRelationExists pins the reason the
// migration's step order is what it is. PocketBase refuses to delete a
// collection that still has relation references pointing at it, so
// thing_type_operations.schema has to be removed first. Without this test the
// ordering comment in the migration is an assertion nobody checked, and a
// well-meaning reorder would fail only on databases that still had the old
// schema -- which is every real one, and none of the test ones.
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
