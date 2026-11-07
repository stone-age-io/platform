// Package migrations_test tests the migrations against a real PocketBase,
// including what a re-import of schema.json does to live collections.
//
// It is an external test package because internal/testutil imports
// platform/migrations; an in-package test could not import the harness.
package migrations_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// probeCollection and probeField name any real, non-system field. subject_prefix
// is load-bearing, so unlikely to be removed; if it is, the precondition fails.
const (
	probeCollection = "thing_types"
	probeField      = "subject_prefix"
)

// TestReimportCannotRemoveAField pins the trap: with deleteMissing=false,
// core.ImportCollections re-adds every live field the import did not carry by
// id, so a re-import logs its ✅ and the column stays. If this fails, the
// explicit RemoveById calls in the removal migrations are no longer needed.
func TestReimportCannotRemoveAField(t *testing.T) {
	app := testutil.SetupApp(t)

	col, err := app.FindCollectionByNameOrId(probeCollection)
	if err != nil {
		t.Fatalf("find %s: %v", probeCollection, err)
	}
	if col.Fields.GetByName(probeField) == nil {
		t.Fatalf("precondition failed: %s.%s is not in the freshly imported schema, "+
			"so this test cannot prove anything about removing it", probeCollection, probeField)
	}

	stripped := schemaWithoutField(t, probeCollection, probeField)
	if err := app.ImportCollectionsByMarshaledJSON(stripped, false); err != nil {
		t.Fatalf("re-import: %v", err)
	}

	col, err = app.FindCollectionByNameOrId(probeCollection)
	if err != nil {
		t.Fatalf("re-find %s: %v", probeCollection, err)
	}
	if col.Fields.GetByName(probeField) == nil {
		t.Fatalf("PocketBase's import semantics have CHANGED: a re-import with "+
			"deleteMissing=false removed %s.%s, which it did not do in v0.39.11. "+
			"The explicit Fields.RemoveById calls in the removal migrations are now "+
			"redundant -- verify against a real install before deleting them.",
			probeCollection, probeField)
	}
	if !tableHasColumn(t, app, probeCollection, probeField) {
		t.Fatalf("field definition survived the re-import but the %s column did not, "+
			"which is a worse state than either outcome this test expects", probeField)
	}
}

// TestExplicitFieldRemovalDropsTheColumn is the other half: RemoveById + Save
// drops the SQLite column, not only the definition.
func TestExplicitFieldRemovalDropsTheColumn(t *testing.T) {
	app := testutil.SetupApp(t)

	col, err := app.FindCollectionByNameOrId(probeCollection)
	if err != nil {
		t.Fatalf("find %s: %v", probeCollection, err)
	}
	f := col.Fields.GetByName(probeField)
	if f == nil {
		t.Fatalf("precondition failed: %s.%s is not in the freshly imported schema", probeCollection, probeField)
	}

	// By id, not by name: a name matches whatever is live under it.
	col.Fields.RemoveById(f.GetId())
	if err := app.Save(col); err != nil {
		t.Fatalf("save %s after removal: %v", probeCollection, err)
	}

	col, err = app.FindCollectionByNameOrId(probeCollection)
	if err != nil {
		t.Fatalf("re-find %s: %v", probeCollection, err)
	}
	if col.Fields.GetByName(probeField) != nil {
		t.Errorf("%s.%s definition survived an explicit RemoveById + Save", probeCollection, probeField)
	}
	if tableHasColumn(t, app, probeCollection, probeField) {
		t.Errorf("%s.%s definition was removed but the SQLite column remains", probeCollection, probeField)
	}
}

// schemaWithoutField returns the repo's schema.json with one field stripped from
// one collection.
func schemaWithoutField(t *testing.T, collection, field string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "schema.json"))
	if err != nil {
		t.Fatalf("read schema.json: %v", err)
	}

	var cols []map[string]any
	if err := json.Unmarshal(raw, &cols); err != nil {
		t.Fatalf("unmarshal schema.json: %v", err)
	}

	stripped := false
	for _, c := range cols {
		if c["name"] != collection {
			continue
		}
		fields, ok := c["fields"].([]any)
		if !ok {
			t.Fatalf("collection %s has no fields array", collection)
		}
		kept := make([]any, 0, len(fields))
		for _, raw := range fields {
			f, ok := raw.(map[string]any)
			if ok && f["name"] == field {
				stripped = true
				continue
			}
			kept = append(kept, raw)
		}
		c["fields"] = kept
	}
	if !stripped {
		t.Fatalf("schema.json has no %s.%s to strip", collection, field)
	}

	out, err := json.Marshal(cols)
	if err != nil {
		t.Fatalf("marshal stripped schema: %v", err)
	}
	return out
}

// tableHasColumn asks SQLite directly: the collection model and the table can
// disagree.
func tableHasColumn(t *testing.T, app core.App, table, column string) bool {
	t.Helper()

	var names []struct {
		Name string `db:"name"`
	}
	err := app.DB().
		NewQuery("SELECT name FROM pragma_table_info({:table})").
		Bind(map[string]any{"table": table}).
		All(&names)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	if len(names) == 0 {
		t.Fatalf("pragma_table_info(%s) returned no columns", table)
	}

	for _, n := range names {
		if n.Name == column {
			return true
		}
	}
	return false
}
