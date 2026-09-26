package migrations_test

import (
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

// The backfill fills every path from the codes and parents already stored,
// uses the id for a location with no code, and leaves a looped chain blank
// rather than picking which member is the root.
func TestLocationPathBackfill(t *testing.T) {
	app := testutil.SetupApp(t)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}
	owner := newRecord("users")
	owner.Set("email", "backfill-owner@example.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatal(err)
	}
	org := newRecord("organizations")
	org.Set("name", "Backfill")
	org.Set("active", true)
	org.Set("owner", owner.Id)
	if err := app.Save(org); err != nil {
		t.Fatal(err)
	}

	newLocation := func(code, parent string) string {
		rec := newRecord("locations")
		rec.Set("name", code)
		rec.Set("code", code)
		rec.Set("organization", org.Id)
		rec.Set("parent", parent)
		if err := app.Save(rec); err != nil {
			t.Fatalf("save %s: %v", code, err)
		}
		return rec.Id
	}
	campus := newLocation("KC", "")
	building := newLocation("BD-3", campus)
	room := newLocation("RM-204", building)
	loopA := newLocation("LOOP-A", "")
	loopB := newLocation("LOOP-B", loopA)

	exec := func(sql string, params dbx.Params) {
		t.Helper()
		if _, err := app.DB().NewQuery(sql).Bind(params).Execute(); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// What a database from before the field looks like: no paths, one
	// location that predates generated codes, and a loop made by hand -- the
	// UI never offers a descendant as a parent, but nothing below it stopped one.
	exec("UPDATE {{locations}} SET [[path]] = ''", nil)
	exec("UPDATE {{locations}} SET [[code]] = '' WHERE [[id]] = {:id}", dbx.Params{"id": building})
	exec("UPDATE {{locations}} SET [[parent]] = {:b} WHERE [[id]] = {:a}", dbx.Params{"a": loopA, "b": loopB})

	if err := migrations.ApplyLocationPath(app); err != nil {
		t.Fatalf("migration: %v", err)
	}

	for id, want := range map[string]string{
		campus:   "/KC/",
		building: "/KC/" + building + "/",
		room:     "/KC/" + building + "/RM-204/",
		loopA:    "",
		loopB:    "",
	} {
		var got string
		if err := app.DB().NewQuery("SELECT [[path]] FROM {{locations}} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"id": id}).Row(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("path of %s = %q, want %q", id, got, want)
		}
	}
}
