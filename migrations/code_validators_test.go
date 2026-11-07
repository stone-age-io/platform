package migrations_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// patterned names the four collections the migration patterns, keyed by field
// id: the migration only works if the id in schema.json matches the live one,
// and a lookup by name would not notice a mismatch.
var patterned = []struct {
	collection string
	id         string
}{
	{"things", "text1997877400"},
	{"locations", "text1997877400"},
	{"thing_types", "text1997877400"},
	{"location_types", "text1997877400"},
}

// restorePreValidatorSchema puts the unpatterned `code` field back through the
// real importer, so the state under test matches a live database.
func restorePreValidatorSchema(t *testing.T, app *pocketbase.PocketBase) {
	t.Helper()

	fixture, err := os.ReadFile(filepath.Join("testdata", "pre_code_validators.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := app.ImportCollectionsByMarshaledJSON(fixture, false); err != nil {
		t.Fatalf("restore pre-validator schema: %v", err)
	}

	for _, p := range patterned {
		f := codeField(t, app, p.collection, p.id)
		if f.Pattern != "" || f.Max != 0 {
			t.Fatalf("fixture did not restore %s.code to its unpatterned shape: pattern=%q max=%d",
				p.collection, f.Pattern, f.Max)
		}
	}
}

func codeField(t *testing.T, app core.App, collection, id string) *core.TextField {
	t.Helper()

	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatalf("find %s: %v", collection, err)
	}
	raw := col.Fields.GetById(id)
	if raw == nil {
		t.Fatalf("%s has no field with id %s", collection, id)
	}
	f, ok := raw.(*core.TextField)
	if !ok {
		t.Fatalf("%s.code is %T, not a text field", collection, raw)
	}
	return f
}

// TestCodeValidatorsReachAnExistingField is the test `migrate up` cannot
// provide: a fresh database already has the constraint. It also checks the
// re-import trap in migrations/CLAUDE.md: the import only replaces the live
// field because `text1997877400` is PocketBase's deterministic id for `code`.
func TestCodeValidatorsReachAnExistingField(t *testing.T) {
	app := testutil.SetupApp(t)
	restorePreValidatorSchema(t, app)

	if err := migrations.ApplyCodeValidators(app); err != nil {
		t.Fatalf("ApplyCodeValidators: %v", err)
	}

	for _, p := range patterned {
		f := codeField(t, app, p.collection, p.id)
		if f.Pattern != migrations.CodePattern.String() {
			t.Errorf("%s.code pattern = %q, want %q", p.collection, f.Pattern, migrations.CodePattern.String())
		}
		if f.Max != 63 {
			t.Errorf("%s.code max = %d, want 63", p.collection, f.Max)
		}
	}
}

// TestCodeValidatorIsEnforcedOnSave proves PocketBase refuses the write, not
// just that the definition landed. The rejected values are the ones with
// consequences (see CodePattern).
func TestCodeValidatorIsEnforcedOnSave(t *testing.T) {
	app := testutil.SetupApp(t)

	col, err := app.FindCollectionByNameOrId("thing_types")
	if err != nil {
		t.Fatalf("find thing_types: %v", err)
	}

	rejected := map[string]string{
		"a dot splits the subject token": "cam.042",
		"the NATS one-token wildcard":    "*",
		"a wildcard inside a code":       "cam-*",
		"the NATS tail wildcard":         "cam>",
		"a space breaks the domain name": "cam 042",
		"a leading hyphen":               "-cam042",
		"longer than the RFC 1123 label": strings.Repeat("c", 64),
		"a slash, which MQTT would fold": "cam/042",
		"a dollar, reserved by NATS":     "$JS",
	}
	for why, code := range rejected {
		r := core.NewRecord(col)
		r.Set("name", "probe")
		r.Set("code", code)
		if err := app.Save(r); err == nil {
			t.Errorf("thing_types accepted code %q (%s)", code, why)
		}
	}

	accepted := map[string]string{
		"the stencilled uppercase form": "KC-DC1",
		"the slug form":                 "access-door",
		"an underscore":                 "edge_gateway",
		"digits throughout":             "816",
		"exactly the length limit":      strings.Repeat("c", 63),
	}
	for why, code := range accepted {
		r := core.NewRecord(col)
		r.Set("name", "probe")
		r.Set("code", code)
		if err := app.Save(r); err != nil {
			t.Errorf("thing_types rejected code %q (%s): %v", code, why, err)
		}
	}

	// Blank stays legal: `code` is optional and the unique index is partial
	// (ADR 0002).
	r := core.NewRecord(col)
	r.Set("name", "probe")
	r.Set("code", "")
	if err := app.Save(r); err != nil {
		t.Errorf("thing_types rejected a blank code: %v", err)
	}
}

// TestNonConformingCodesReportsRatherThanRewrites pins that the migration does
// not backfill: `code` is frozen (schema_update_inventory_code_freeze.go), so it
// reports offenders and leaves the fix to a human.
func TestNonConformingCodesReportsRatherThanRewrites(t *testing.T) {
	app := testutil.SetupApp(t)
	restorePreValidatorSchema(t, app)

	col, err := app.FindCollectionByNameOrId("thing_types")
	if err != nil {
		t.Fatalf("find thing_types: %v", err)
	}
	bad := core.NewRecord(col)
	bad.Set("name", "legacy")
	bad.Set("code", "cam.042")
	if err := app.Save(bad); err != nil {
		t.Fatalf("the unpatterned collection should have accepted %q: %v", "cam.042", err)
	}

	if err := migrations.ApplyCodeValidators(app); err != nil {
		t.Fatalf("ApplyCodeValidators: %v", err)
	}

	offenders, err := migrations.NonConformingCodes(app, "thing_types")
	if err != nil {
		t.Fatalf("NonConformingCodes: %v", err)
	}
	if len(offenders) != 1 || offenders[0].Code != "cam.042" {
		t.Fatalf("NonConformingCodes = %+v, want exactly the one legacy code", offenders)
	}

	// The record is still readable and its code unchanged.
	reloaded, err := app.FindRecordById("thing_types", bad.Id)
	if err != nil {
		t.Fatalf("the non-conforming record became unreadable: %v", err)
	}
	if got := reloaded.GetString("code"); got != "cam.042" {
		t.Errorf("the migration rewrote a frozen code to %q", got)
	}
}
