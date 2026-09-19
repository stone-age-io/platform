package migrations_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// patterned names the four collections the migration is responsible for and the
// id it keys on. Keyed by id for the reason drop_contract_layer_test.go is: a
// lookup by name would pass against a collection rebuilt with different ids and
// prove nothing, and this migration's whole premise is that the id in
// schema.json matches the live one so a re-import actually replaces the field.
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
// REAL importer, so the state under test is the state a live database is in
// rather than one assembled field by field in Go.
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
// provide. A fresh database imports the already-patterned schema.json, so the
// migration runs against a field that already has the constraint and passes
// without changing anything.
//
// It is also the specific check CLAUDE.md demands before trusting any
// re-import migration: PocketBase's importer re-adds every live field the
// import did not carry BY ID, and FieldsList.add then keeps the live definition
// over the imported one. A field whose id in schema.json does not match the
// live id is therefore silently left alone while the migration logs its tick.
// `text1997877400` is PocketBase's deterministic id for a field named `code`
// (the type prefix plus crc32 of the name), which is why this works — assert it
// rather than assume it.
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

// TestCodeValidatorIsEnforcedOnSave proves the pattern is a constraint rather
// than a decoration. The definition landing in the collection is necessary but
// not sufficient — what matters is that PocketBase refuses the write.
//
// The rejected values are the ones with consequences, not arbitrary bad input:
// a dot splits one NATS token into two, `*` and `>` are the NATS wildcards that
// would widen a generated permission pattern from one identity to every
// identity, and a space breaks the JetStream domain an edge site's code becomes.
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

	// Blank stays legal. `code` is optional on all four collections and the
	// unique index is partial, so a record without one is an ordinary state --
	// the platform's pre-onboarding inventory and a provider's service work both
	// cover gear with no upstream record (ADR 0002).
	r := core.NewRecord(col)
	r.Set("name", "probe")
	r.Set("code", "")
	if err := app.Save(r); err != nil {
		t.Errorf("thing_types rejected a blank code: %v", err)
	}
}

// TestNonConformingCodesReportsRatherThanRewrites pins the migration's one
// deliberate omission. Every other constraint-adding migration here backfills;
// this one refuses to, because `code` is frozen
// (schema_update_inventory_code_freeze.go) precisely so that nothing rewrites a
// string that is printed on a label and baked into a signed subject. Rewriting
// codes at scale to satisfy a new validator is the exact act the freeze exists
// to prevent, so the migration reports and leaves the decision to a human.
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

	// The record survived: readable, un-rewritten, and still carrying the code
	// someone screwed to a wall.
	reloaded, err := app.FindRecordById("thing_types", bad.Id)
	if err != nil {
		t.Fatalf("the non-conforming record became unreadable: %v", err)
	}
	if got := reloaded.GetString("code"); got != "cam.042" {
		t.Errorf("the migration rewrote a frozen code to %q", got)
	}
}
