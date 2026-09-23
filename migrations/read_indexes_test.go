package migrations_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

var addedReadIndexes = map[string][]string{
	"things":     {"idx_things_org", "idx_things_location"},
	"locations":  {"idx_locations_org"},
	"nats_users": {"idx_nats_users_role"},
}

const droppedMembershipIndex = "idx_memberships_user"

func tableIndexes(t *testing.T, app core.App, table string) map[string]bool {
	t.Helper()

	var rows []struct {
		Name string `db:"name"`
	}
	err := app.DB().
		NewQuery("SELECT name FROM pragma_index_list({:table})").
		Bind(map[string]any{"table": table}).
		All(&rows)
	if err != nil {
		t.Fatalf("pragma_index_list(%s): %v", table, err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[r.Name] = true
	}
	return out
}

// restorePreReadIndexShape puts the database back to how it looked before the
// migration. A fresh database imports the schema.json that already has the new
// indexes, so without this the migration runs against nothing and passes for
// the wrong reason.
func restorePreReadIndexShape(t *testing.T, app core.App) {
	t.Helper()

	for table, names := range addedReadIndexes {
		col, err := app.FindCollectionByNameOrId(table)
		if err != nil {
			t.Fatalf("find %s: %v", table, err)
		}
		kept := col.Indexes[:0]
		for _, idx := range col.Indexes {
			drop := false
			for _, n := range names {
				if strings.Contains(idx, "`"+n+"`") {
					drop = true
				}
			}
			if !drop {
				kept = append(kept, idx)
			}
		}
		col.Indexes = kept
		if err := app.Save(col); err != nil {
			t.Fatalf("restore %s: %v", table, err)
		}
	}

	col, err := app.FindCollectionByNameOrId("memberships")
	if err != nil {
		t.Fatalf("find memberships: %v", err)
	}
	if col.GetIndex(droppedMembershipIndex) == "" {
		col.AddIndex(droppedMembershipIndex, false, "user", "")
		if err := app.Save(col); err != nil {
			t.Fatalf("restore memberships: %v", err)
		}
	}

	// Confirm the fixture took, or the assertions below prove nothing.
	for table, names := range addedReadIndexes {
		live := tableIndexes(t, app, table)
		for _, n := range names {
			if live[n] {
				t.Fatalf("fixture left %s on %s", n, table)
			}
		}
	}
	if !tableIndexes(t, app, "memberships")[droppedMembershipIndex] {
		t.Fatalf("fixture did not restore %s", droppedMembershipIndex)
	}
}

// TestReadIndexes checks the live tables, not the collection JSON: an index
// listed in schema.json that never reached SQLite is the failure worth catching.
func TestReadIndexes(t *testing.T) {
	app := testutil.SetupApp(t)

	restorePreReadIndexShape(t, app)

	if err := migrations.AddReadIndexes(app); err != nil {
		t.Fatalf("AddReadIndexes: %v", err)
	}

	for table, names := range addedReadIndexes {
		live := tableIndexes(t, app, table)
		for _, n := range names {
			if !live[n] {
				t.Errorf("%s: index %s missing after migration", table, n)
			}
		}
	}

	live := tableIndexes(t, app, "memberships")
	if live[droppedMembershipIndex] {
		t.Errorf("memberships: %s still present after migration", droppedMembershipIndex)
	}
	// The composite that makes the single-column index redundant must survive.
	if !live["idx_memberships_user_org"] {
		t.Errorf("memberships: idx_memberships_user_org missing")
	}
}

// TestOrgScopedThingReadUsesAnIndex is the reason for the migration. The
// partial UNIQUE (organization, code) index, over non-blank codes, cannot
// serve this query, so without idx_things_org it is a full scan.
func TestOrgScopedThingReadUsesAnIndex(t *testing.T) {
	app := testutil.SetupApp(t)

	var plan []struct {
		Detail string `db:"detail"`
	}
	err := app.DB().
		NewQuery("EXPLAIN QUERY PLAN SELECT * FROM things WHERE organization = {:o} ORDER BY created DESC LIMIT 30").
		Bind(map[string]any{"o": "x"}).
		All(&plan)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}

	for _, p := range plan {
		if strings.Contains(p.Detail, "INDEX idx_things_org ") {
			return
		}
	}
	t.Errorf("org-scoped things read does not use idx_things_org; plan: %+v", plan)
}
