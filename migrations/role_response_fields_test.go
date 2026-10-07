package migrations_test

import (
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// responseFields are keyed by id, not name: the schema.json ids must be the ones
// PocketBase assigns to the fields pb-nats adds at startup, or a re-import keeps
// the live field and ignores the schema's definition (CLAUDE.md).
var responseFields = map[string]string{
	"bool1483994014":   "allow_response",
	"number1537381278": "allow_response_max",
	"number1698314504": "allow_response_ttl",
}

// A fresh database already has these fields -- pb-nats creates nats_roles before
// schema.json is imported, and a re-import never removes a field -- so testing
// the end state of a fresh app passes with or without this migration. The shape
// at risk is a database whose nats_roles predates pb-nats adding them, so the
// test builds that shape first, proves a write to it is dropped, and only then
// applies the migration.
func TestRoleResponseFieldsRoundTrip(t *testing.T) {
	app := testutil.SetupApp(t)

	col, err := app.FindCollectionByNameOrId("nats_roles")
	if err != nil {
		t.Fatalf("find nats_roles: %v", err)
	}
	for _, name := range responseFields {
		if f := col.Fields.GetByName(name); f != nil {
			col.Fields.RemoveById(f.GetId())
		}
	}
	if err := app.Save(col); err != nil {
		t.Fatalf("strip response fields: %v", err)
	}

	// The premise: in that shape a write reports success and stores nothing.
	if saveResponder(t, app).GetBool("allow_response") {
		t.Fatal("legacy shape kept allow_response; the test is not reproducing the bug")
	}

	// What the migration does.
	if err := app.ImportCollectionsByMarshaledJSON(migrations.SchemaJSON, false); err != nil {
		t.Fatalf("import schema.json: %v", err)
	}

	col, err = app.FindCollectionByNameOrId("nats_roles")
	if err != nil {
		t.Fatalf("find nats_roles: %v", err)
	}
	for id, name := range responseFields {
		f := col.Fields.GetById(id)
		if f == nil {
			t.Fatalf("nats_roles has no field with id %s (%s)", id, name)
		}
		if f.GetName() != name {
			t.Fatalf("field %s is named %q, want %q", id, f.GetName(), name)
		}
	}

	got := saveResponder(t, app)
	if !got.GetBool("allow_response") {
		t.Error("allow_response did not survive a save: the write was dropped")
	}
	if v := got.GetInt("allow_response_max"); v != -1 {
		t.Errorf("allow_response_max = %d, want -1", v)
	}
	if v := got.GetInt("allow_response_ttl"); v != 30 {
		t.Errorf("allow_response_ttl = %d, want 30", v)
	}
}

// saveResponder writes a role with response permissions and returns it as
// RELOADED from the database, since the failure is a save that reports success.
func saveResponder(t *testing.T, app *pocketbase.PocketBase) *core.Record {
	t.Helper()

	col, err := app.FindCollectionByNameOrId("nats_roles")
	if err != nil {
		t.Fatalf("find nats_roles: %v", err)
	}
	role := core.NewRecord(col)
	role.Set("name", "responder")
	role.Set("allow_response", true)
	role.Set("allow_response_max", -1)
	role.Set("allow_response_ttl", 30)
	if err := app.Save(role); err != nil {
		t.Fatalf("save role: %v", err)
	}
	got, err := app.FindRecordById("nats_roles", role.Id)
	if err != nil {
		t.Fatalf("reload role: %v", err)
	}
	return got
}
