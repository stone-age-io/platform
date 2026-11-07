package migrations_test

import (
	"sort"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// protectedFileFields are the file fields that existed when the migration was
// written, spelled out so the test does not derive its expectation the same way
// the migration does.
var protectedFileFields = []struct{ collection, field string }{
	{"users", "avatar"},
	{"organizations", "logo"},
	{"locations", "floorplan"},
}

// unprotect puts a field back to the pre-migration state so the migration has
// something to do; a fresh database is already protected.
func unprotect(t *testing.T, app *pocketbase.PocketBase, collection, field string) {
	t.Helper()

	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatalf("find %s: %v", collection, err)
	}
	f, ok := col.Fields.GetByName(field).(*core.FileField)
	if !ok {
		t.Fatalf("%s.%s is not a file field", collection, field)
	}
	f.Protected = false
	if err := app.Save(col); err != nil {
		t.Fatalf("unprotect %s.%s: %v", collection, field, err)
	}
}

func protectedFlag(t *testing.T, app *pocketbase.PocketBase, collection, field string) bool {
	t.Helper()

	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatalf("find %s: %v", collection, err)
	}
	f, ok := col.Fields.GetByName(field).(*core.FileField)
	if !ok {
		t.Fatalf("%s.%s is not a file field", collection, field)
	}
	return f.Protected
}

// TestProtectFileFieldsProtectsEveryUploadField is the test `migrate up` cannot
// provide. It unprotects all three first, so the flip is what is under test.
func TestProtectFileFieldsProtectsEveryUploadField(t *testing.T) {
	app := testutil.SetupApp(t)

	for _, f := range protectedFileFields {
		unprotect(t, app, f.collection, f.field)
		if protectedFlag(t, app, f.collection, f.field) {
			t.Fatalf("fixture did not unprotect %s.%s", f.collection, f.field)
		}
	}

	if err := migrations.ProtectFileFields(app); err != nil {
		t.Fatalf("ProtectFileFields: %v", err)
	}

	for _, f := range protectedFileFields {
		if !protectedFlag(t, app, f.collection, f.field) {
			t.Errorf("%s.%s was not protected", f.collection, f.field)
		}
	}
}

// TestProtectFileFieldsIsIdempotent covers the fresh-database path (nothing to
// do) and a replayed migration set.
func TestProtectFileFieldsIsIdempotent(t *testing.T) {
	app := testutil.SetupApp(t)

	for i := 0; i < 2; i++ {
		if err := migrations.ProtectFileFields(app); err != nil {
			t.Fatalf("ProtectFileFields run %d: %v", i+1, err)
		}
	}
	for _, f := range protectedFileFields {
		if !protectedFlag(t, app, f.collection, f.field) {
			t.Errorf("%s.%s lost its protection across runs", f.collection, f.field)
		}
	}
}

// TestProtectFileFieldsAppliesWhenTheLiveFieldIdDiffers is why the migration
// mutates the field instead of re-importing schema.json: a re-import keeps the
// live field when the ids differ (the re-import note in migrations/CLAUDE.md).
// It fails if the migration is turned into an ImportCollectionsByMarshaledJSON
// call.
func TestProtectFileFieldsAppliesWhenTheLiveFieldIdDiffers(t *testing.T) {
	app := testutil.SetupApp(t)

	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("find users: %v", err)
	}
	f, ok := col.Fields.GetByName("avatar").(*core.FileField)
	if !ok {
		t.Fatal("users.avatar is not a file field")
	}
	// Changing an id is safe: PocketBase reuses the existing field object
	// rather than recreating the column.
	f.SetId("file_avatar_renamed")
	f.Protected = false
	if err := app.Save(col); err != nil {
		t.Fatalf("re-id users.avatar: %v", err)
	}

	if err := migrations.ProtectFileFields(app); err != nil {
		t.Fatalf("ProtectFileFields: %v", err)
	}
	if !protectedFlag(t, app, "users", "avatar") {
		t.Error("users.avatar was not protected when its live id differed from schema.json")
	}
}

// TestEveryFileFieldInTheSchemaIsProtected walks the live schema and fails on
// any unprotected file field, so a new upload field that forgets the flag is a
// red test. If a public upload field is ever wanted, state that decision here.
func TestEveryFileFieldInTheSchemaIsProtected(t *testing.T) {
	app := testutil.SetupApp(t)

	collections, err := app.FindAllCollections()
	if err != nil {
		t.Fatalf("find all collections: %v", err)
	}

	var unprotected []string
	var found int
	for _, col := range collections {
		if col.System {
			continue // PocketBase internals
		}
		for _, field := range col.Fields {
			f, ok := field.(*core.FileField)
			if !ok {
				continue
			}
			found++
			if !f.Protected {
				unprotected = append(unprotected, col.Name+"."+f.Name)
			}
		}
	}

	if found == 0 {
		t.Fatal("found no file fields at all; this test would pass vacuously")
	}
	if len(unprotected) > 0 {
		sort.Strings(unprotected)
		t.Errorf("unprotected file field(s): %v\n"+
			"An unprotected file is served by /api/files to anyone with the URL, with no\n"+
			"auth and no expiry. Set \"protected\": true on it in schema.json\n"+
			"(schema_update_protected_files.go derives its list from the live schema).",
			unprotected)
	}
}
