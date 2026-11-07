package migrations_test

import (
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"platform/internal/testutil"
	"platform/migrations"
)

// restoreLeafNodes puts the leaf_nodes collection back through the real
// importer, since a fresh database never has it and `migrate up` would test
// nothing.
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

	// The table too, not just the definition.
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

// Covers a fresh database and a replay of the migration set.
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

// Nothing relates into leaf_nodes, so DropLeafNodes can delete it outright. If
// this fails, the migration needs DropContractLayer's relation-first order.
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
