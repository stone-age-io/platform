package migrations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketbase/pocketbase"

	"platform/internal/testutil"
	"platform/migrations"
)

// restoreLeafNodes puts the leaf_nodes collection back through the REAL importer,
// so the state under test is the state a live database is actually in rather than
// one assembled collection-by-collection in Go.
//
// Same reason as restorePreDropSchema: a fresh database imports the
// already-thinned schema.json, so `migrate up` reaches DropLeafNodes with nothing
// to remove and passes without touching anything. Without this the migration's
// green tick would mean nothing.
func restoreLeafNodes(t *testing.T, app *pocketbase.PocketBase) {
	t.Helper()

	fixture, err := os.ReadFile(filepath.Join("testdata", "pre_drop_leaf_nodes.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := app.ImportCollectionsByMarshaledJSON(fixture, false); err != nil {
		t.Fatalf("restore leaf_nodes: %v", err)
	}
	if _, err := app.FindCollectionByNameOrId("leaf_nodes"); err != nil {
		t.Fatalf("fixture did not restore the leaf_nodes collection: %v", err)
	}
	if !tableHasColumn(t, app, "leaf_nodes", "domain") {
		t.Fatal("fixture restored the leaf_nodes definition but not its table")
	}
}

func TestDropLeafNodesRemovesTheCollection(t *testing.T) {
	app := testutil.SetupApp(t)
	restoreLeafNodes(t, app)

	if err := migrations.DropLeafNodes(app); err != nil {
		t.Fatalf("DropLeafNodes: %v", err)
	}

	if col, err := app.FindCollectionByNameOrId("leaf_nodes"); err == nil && col != nil {
		t.Error("leaf_nodes collection survived the migration")
	}

	// The table too, not just the definition. PocketBase drops both, but this is
	// the half a definition-only assertion would miss.
	var tables []struct {
		Name string `db:"name"`
	}
	if err := app.DB().
		NewQuery("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'leaf_nodes'").
		All(&tables); err != nil {
		t.Fatalf("sqlite_master: %v", err)
	}
	if len(tables) != 0 {
		t.Error("the leaf_nodes table survived the migration")
	}
}

// Covers the path a fresh database takes, and a replay of the migration set.
func TestDropLeafNodesIsIdempotent(t *testing.T) {
	app := testutil.SetupApp(t)

	if err := migrations.DropLeafNodes(app); err != nil {
		t.Fatalf("DropLeafNodes on a database that never had it: %v", err)
	}
	restoreLeafNodes(t, app)
	if err := migrations.DropLeafNodes(app); err != nil {
		t.Fatalf("DropLeafNodes after restore: %v", err)
	}
	if err := migrations.DropLeafNodes(app); err != nil {
		t.Fatalf("DropLeafNodes second run: %v", err)
	}
}

// Nothing relates INTO leaf_nodes, which is why DropLeafNodes can delete it
// outright where DropContractLayer had to remove a relation first. If a relation
// into it is ever reintroduced, PocketBase will refuse the delete and the
// migration needs the same two-step shape -- this is the test that says so
// instead of a comment nobody checked.
func TestNothingRelatesIntoLeafNodes(t *testing.T) {
	app := testutil.SetupApp(t)
	restoreLeafNodes(t, app)

	col, err := app.FindCollectionByNameOrId("leaf_nodes")
	if err != nil {
		t.Fatalf("find leaf_nodes: %v", err)
	}
	if err := app.Delete(col); err != nil {
		t.Fatalf("deleting leaf_nodes was refused, so something now references it: %v", err)
	}
}
