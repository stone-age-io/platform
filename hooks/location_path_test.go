package hooks_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// TestLocationPath covers ADR 0004's locations.path against a real PocketBase:
// the path on create, the subtree rewrite on a move, the cycle refusal, and what
// a deleted parent does to its children -- the last being PocketBase's relation
// cleanup firing our update hook, which is exactly the kind of behaviour worth
// pinning in case an upgrade changes it.
func TestLocationPath(t *testing.T) {
	app := testutil.SetupApp(t)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}
	mustSave := func(t *testing.T, rec *core.Record) *core.Record {
		t.Helper()
		if err := app.Save(rec); err != nil {
			t.Fatalf("save %s: %v", rec.GetString("code"), err)
		}
		return rec
	}
	pathOf := func(t *testing.T, rec *core.Record) string {
		t.Helper()
		fresh, err := app.FindRecordById("locations", rec.Id)
		if err != nil {
			t.Fatalf("reload %s: %v", rec.GetString("code"), err)
		}
		return fresh.GetString("path")
	}

	owner := newRecord("users")
	owner.Set("email", "path-owner@example.test")
	owner.Set("password", "owner-password-123")
	mustSave(t, owner)

	newOrg := func(name string) *core.Record {
		rec := newRecord("organizations")
		rec.Set("name", name)
		rec.Set("active", true)
		rec.Set("owner", owner.Id)
		return mustSave(t, rec)
	}
	org := newOrg("Path Alpha")
	other := newOrg("Path Beta")

	newLocation := func(code string, parent *core.Record, orgRec *core.Record) *core.Record {
		rec := newRecord("locations")
		rec.Set("name", code)
		rec.Set("code", code)
		rec.Set("organization", orgRec.Id)
		if parent != nil {
			rec.Set("parent", parent.Id)
		}
		return rec
	}

	campus := mustSave(t, newLocation("KC", nil, org))
	building := mustSave(t, newLocation("BD-3", campus, org))
	room := mustSave(t, newLocation("RM-204", building, org))
	// A near-miss of BD-3 and a code with `_`, the LIKE wildcard. Neither may
	// be touched by rewrites aimed at their neighbours.
	building30 := mustSave(t, newLocation("BD-30", campus, org))
	underscore := mustSave(t, newLocation("A_1", campus, org))
	lookalike := mustSave(t, newLocation("AB1", campus, org))
	lookalikeRoom := mustSave(t, newLocation("RM-9", lookalike, org))

	t.Run("create builds the path from the root down", func(t *testing.T) {
		for rec, want := range map[*core.Record]string{
			campus:   "/KC/",
			building: "/KC/BD-3/",
			room:     "/KC/BD-3/RM-204/",
		} {
			if got := pathOf(t, rec); got != want {
				t.Errorf("%s path = %q, want %q", rec.GetString("code"), got, want)
			}
		}
	})

	t.Run("a generated code is in the path", func(t *testing.T) {
		rec := mustSave(t, newLocation("", campus, org))
		code := rec.GetString("code")
		if code == "" || pathOf(t, rec) != "/KC/"+code+"/" {
			t.Fatalf("path = %q for generated code %q", pathOf(t, rec), code)
		}
	})

	t.Run("a client-sent path is overwritten", func(t *testing.T) {
		rec := newLocation("RM-205", building, org)
		rec.Set("path", "/forged/")
		mustSave(t, rec)
		if got := pathOf(t, rec); got != "/KC/BD-3/RM-205/" {
			t.Fatalf("path = %q, want the computed one", got)
		}
	})

	t.Run("moving a location moves everything under it and nothing else", func(t *testing.T) {
		building.Set("parent", underscore.Id)
		mustSave(t, building)
		if got := pathOf(t, building); got != "/KC/A_1/BD-3/" {
			t.Fatalf("moved building path = %q", got)
		}
		if got := pathOf(t, room); got != "/KC/A_1/BD-3/RM-204/" {
			t.Fatalf("room under the moved building = %q", got)
		}
		if got := pathOf(t, building30); got != "/KC/BD-30/" {
			t.Fatalf("BD-30 was rewritten by a move of BD-3: %q", got)
		}

		// Moving A_1 must not rewrite AB1's subtree, which LIKE '/KC/A_1/%' would.
		underscore.Set("parent", "")
		mustSave(t, underscore)
		if got := pathOf(t, room); got != "/A_1/BD-3/RM-204/" {
			t.Fatalf("room two levels under the moved A_1 = %q", got)
		}
		if got := pathOf(t, lookalikeRoom); got != "/KC/AB1/RM-9/" {
			t.Fatalf("AB1's room was rewritten by a move of A_1: %q", got)
		}
	})

	t.Run("a location cannot be moved under itself or its descendants", func(t *testing.T) {
		for _, parent := range []*core.Record{underscore, room} {
			fresh, _ := app.FindRecordById("locations", underscore.Id)
			fresh.Set("parent", parent.Id)
			err := app.Save(fresh)
			if err == nil {
				t.Fatalf("A_1 under %s saved; want a refusal", parent.GetString("code"))
			}
			if !strings.Contains(err.Error(), "own") {
				t.Fatalf("refusal = %v, want one naming the cycle", err)
			}
		}
		if got := pathOf(t, underscore); got != "/A_1/" {
			t.Fatalf("path after refused moves = %q", got)
		}
	})

	t.Run("a parent in another organization is refused", func(t *testing.T) {
		foreign := mustSave(t, newLocation("ELSEWHERE", nil, other))
		err := app.Save(newLocation("RM-1", foreign, org))
		if err == nil {
			t.Fatal("a location under another organization's location saved")
		}
	})

	t.Run("deleting a parent makes its children roots, subtrees and all", func(t *testing.T) {
		if err := app.Delete(underscore); err != nil {
			t.Fatalf("delete A_1: %v", err)
		}
		if got := pathOf(t, building); got != "/BD-3/" {
			t.Fatalf("orphaned building path = %q, want /BD-3/", got)
		}
		if got := pathOf(t, room); got != "/BD-3/RM-204/" {
			t.Fatalf("room under the orphaned building = %q", got)
		}
	})
}
