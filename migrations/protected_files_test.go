package migrations_test

import (
	"sort"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// protectedFileFields is the list this migration is responsible for, repeated
// here on purpose rather than exported from the migration. A test that read the
// migration's own list could only ever confirm the migration agrees with
// itself; spelling it out means changing the migration and forgetting the test
// is a failure rather than a silent agreement.
var protectedFileFields = []struct{ collection, field string }{
	{"users", "avatar"},
	{"organizations", "logo"},
	{"locations", "floorplan"},
}

// unprotect puts a field back to the pre-migration state so the migration has
// something to do. A fresh database imports a schema.json that already carries
// `"protected": true`, so without this the migration runs against nothing and
// passes for the wrong reason -- the same trap drop_contract_layer_test.go
// exists to avoid.
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

// TestProtectFileFieldsAppliesWhenTheLiveFieldIdDiffers is the reason this
// migration mutates the field instead of re-importing schema.json.
//
// core.ImportCollections re-adds every live field the import did not carry BY
// ID, and FieldsList.add then keeps the EXISTING definition -- so a re-import
// against a database whose field id does not match schema.json logs its ✅ and
// changes nothing. That has bitten this repo twice already (see the re-import
// note in CLAUDE.md). For a mime-type list it is survivable. For the flag that
// decides whether a file is served to unauthenticated strangers it is not.
//
// This drives a live field's id away from the one schema.json carries and
// asserts the migration still applies. It fails the moment anyone "simplifies"
// this migration into an ImportCollectionsByMarshaledJSON call.
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

// TestEveryFileFieldInTheSchemaIsProtected is the guard that matters over time.
//
// The migration above fixes the three fields that exist today. This walks the
// LIVE schema and fails on any file field that is not protected, so adding an
// upload field -- a device photo, a site photo, an attachment -- and forgetting
// to protect it is a red test rather than a quiet hole. It deliberately does
// not consult the migration's list: a new field is a finding whether or not
// anyone remembered to enumerate it.
//
// If a genuinely public upload field is ever wanted, this test is the place to
// state that decision and say why, rather than the flag quietly being false.
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
			continue // PocketBase internals are not ours to re-declare
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
			"auth and no expiry. Add the field to fileFieldsToProtect in\n"+
			"schema_update_protected_files.go and set \"protected\": true in schema.json.",
			unprotected)
	}
}
