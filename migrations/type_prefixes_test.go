package migrations_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// foldedIndexes are the four indexes the migration rebuilds with
// COLLATE NOCASE, and the case-sensitive definition each one replaces.
var foldedIndexes = map[string]string{
	"things":         "CREATE UNIQUE INDEX `idx_things_org_code` ON `things` (`organization`, `code`) WHERE `code` != ''",
	"locations":      "CREATE UNIQUE INDEX `idx_locations_org_code` ON `locations` (`organization`, `code`) WHERE `code` != ''",
	"thing_types":    "CREATE UNIQUE INDEX `idx_thing_types_org_code` ON `thing_types` (`organization`, `code`) WHERE `code` != ''",
	"location_types": "CREATE UNIQUE INDEX `idx_location_types_org_code` ON `location_types` (`organization`, `code`) WHERE `code` != ''",
}

// restoreCaseSensitiveIndexes puts the pre-migration indexes back. A fresh
// database imports the schema.json that already folds case, so without this the
// migration runs against nothing and passes for the wrong reason.
func restoreCaseSensitiveIndexes(t *testing.T, app core.App) {
	t.Helper()
	for table, def := range foldedIndexes {
		col, err := app.FindCollectionByNameOrId(table)
		if err != nil {
			t.Fatalf("find %s: %v", table, err)
		}
		for i, idx := range col.Indexes {
			if strings.Contains(idx, "_org_code`") {
				col.Indexes[i] = def
			}
		}
		if err := app.Save(col); err != nil {
			t.Fatalf("restore %s index: %v", table, err)
		}
	}
}

func liveIndex(t *testing.T, app core.App, table, name string) string {
	t.Helper()
	var sql string
	if err := app.DB().
		NewQuery("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = {:name}").
		Bind(map[string]any{"name": name}).
		Row(&sql); err != nil {
		t.Fatalf("read index %s on %s: %v", name, table, err)
	}
	return sql
}

// The sweep must refuse codes that differ only by case, name them, and leave the
// case-sensitive index standing; once a human corrects one, the same migration
// folds all four indexes.
func TestTypePrefixesRefusesCaseOnlyDuplicates(t *testing.T) {
	app := testutil.SetupApp(t)
	restoreCaseSensitiveIndexes(t, app)

	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	owner := core.NewRecord(col)
	owner.Set("email", "prefix-mig@example.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatal(err)
	}
	orgCol, _ := app.FindCollectionByNameOrId("organizations")
	org := core.NewRecord(orgCol)
	org.Set("name", "Case Sweep")
	org.Set("active", true)
	org.Set("owner", owner.Id)
	if err := app.Save(org); err != nil {
		t.Fatal(err)
	}

	locCol, _ := app.FindCollectionByNameOrId("locations")
	var upper *core.Record
	for _, code := range []string{"hq", "HQ"} {
		loc := core.NewRecord(locCol)
		loc.Set("name", "Site "+code)
		loc.Set("code", code)
		loc.Set("organization", org.Id)
		if err := app.Save(loc); err != nil {
			t.Fatalf("the restored case-sensitive index should admit %q: %v", code, err)
		}
		upper = loc
	}

	err = migrations.ApplyTypePrefixes(app)
	if err == nil {
		t.Fatal("the migration folded case over hq and HQ instead of refusing")
	}
	if !strings.Contains(err.Error(), "hq") || !strings.Contains(err.Error(), "HQ") {
		t.Errorf("the refusal should name both codes, got: %v", err)
	}
	if got := liveIndex(t, app, "locations", "idx_locations_org_code"); strings.Contains(strings.ToUpper(got), "NOCASE") {
		t.Errorf("a refused migration still rebuilt the index: %s", got)
	}

	// The human fix, then the same migration again.
	upper.Set("code", "HQ-2")
	if err := app.Save(upper); err != nil {
		t.Fatal(err)
	}
	if err := migrations.ApplyTypePrefixes(app); err != nil {
		t.Fatalf("migration after the fix: %v", err)
	}
	for table := range foldedIndexes {
		name := "idx_" + table + "_org_code"
		if got := liveIndex(t, app, table, name); !strings.Contains(strings.ToUpper(got), "COLLATE NOCASE") {
			t.Errorf("%s is still case-sensitive after the migration: %s", name, got)
		}
	}
}
