package migrations_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// The view's columns are an authorization decision, not a convenience: its API
// rule replaces the rules of the tables it reads, so every column is readable by
// every operator with no row rule behind it. A name, code or id of a counted
// record added here would hand operators tenant inventory, which they are
// deliberately not given. Change this list only alongside AUTHORIZATION.md.
var orgStatsColumns = []string{"id", "locations", "members", "things", "things_active"}

func TestOrgStatsExposesCountsOnly(t *testing.T) {
	app := testutil.SetupApp(t)

	col, err := app.FindCollectionByNameOrId("org_stats")
	if err != nil {
		t.Fatalf("org_stats collection: %v", err)
	}
	if col.Type != core.CollectionTypeView {
		t.Fatalf("org_stats type = %q, want view", col.Type)
	}

	var names []string
	for _, f := range col.Fields {
		names = append(names, f.GetName())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(orgStatsColumns, ",") {
		t.Fatalf("org_stats columns = %v, want exactly %v", names, orgStatsColumns)
	}

	for _, rule := range []struct {
		name string
		rule *string
	}{{"listRule", col.ListRule}, {"viewRule", col.ViewRule}} {
		if rule.rule == nil || stripRuleComments(*rule.rule) != "@request.auth.is_operator = true" {
			t.Fatalf("org_stats %s must be operator-only, got %v", rule.name, rule.rule)
		}
	}
}

// The counts are the organization asked about. The page this view replaced
// counted the READER's organization and showed 0 for every other one, so the
// fixture has two organizations with different shapes.
func TestOrgStatsCountsTheOrganizationAsked(t *testing.T) {
	app := testutil.SetupApp(t)

	newRecord := func(collection string) *core.Record {
		c, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(c)
	}
	save := func(what string, r *core.Record) {
		t.Helper()
		if err := app.Save(r); err != nil {
			t.Fatalf("save %s: %v", what, err)
		}
	}

	owner := newRecord("users")
	owner.Set("email", "stats-owner@example.test")
	owner.Set("password", "owner-password-123")
	save("owner", owner)

	newOrg := func(name string) *core.Record {
		org := newRecord("organizations")
		org.Set("name", name)
		org.Set("active", true)
		org.Set("owner", owner.Id)
		save(name, org)
		return org
	}
	big, small := newOrg("Big Tenant"), newOrg("Small Tenant")

	n := 0
	newThing := func(org *core.Record, active bool) {
		n++
		thing := newRecord("things")
		thing.Set("organization", org.Id)
		thing.Set("name", fmt.Sprintf("Thing %d", n))
		thing.Set("email", fmt.Sprintf("thing-%d@stats.thing.local", n))
		thing.Set("emailVisibility", true)
		thing.Set("password", "thing-password-123")
		save("thing", thing)
		// A device is born enabled (hooks/active_flag.go forces active on
		// create), so a deactivated one is deactivated afterwards, as in the
		// console.
		if !active {
			thing.Set("active", false)
			save("deactivated thing", thing)
		}
	}
	newLocation := func(org *core.Record) {
		n++
		loc := newRecord("locations")
		loc.Set("organization", org.Id)
		loc.Set("name", fmt.Sprintf("Location %d", n))
		save("location", loc)
	}

	newThing(big, true)
	newThing(big, true)
	newThing(big, false)
	newLocation(big)
	newLocation(big)
	newThing(small, true)

	for _, tc := range []struct {
		org                             *core.Record
		things, thingsActive, locations int
	}{
		{big, 3, 2, 2},
		{small, 1, 1, 0},
	} {
		stats, err := app.FindRecordById("org_stats", tc.org.Id)
		if err != nil {
			t.Fatalf("org_stats row for %s: %v", tc.org.GetString("name"), err)
		}
		// Members are created by the organization hooks, so compare against the
		// table rather than assume how many an organization starts with.
		members, err := app.CountRecords("memberships", dbx.HashExp{"organization": tc.org.Id})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int{
			"things":        stats.GetInt("things"),
			"things_active": stats.GetInt("things_active"),
			"locations":     stats.GetInt("locations"),
			"members":       stats.GetInt("members"),
		}
		want := map[string]int{
			"things":        tc.things,
			"things_active": tc.thingsActive,
			"locations":     tc.locations,
			"members":       int(members),
		}
		for k, w := range want {
			if got[k] != w {
				t.Errorf("%s: %s = %d, want %d", tc.org.GetString("name"), k, got[k], w)
			}
		}
	}
}

func stripRuleComments(rule string) string {
	var kept []string
	for _, line := range strings.Split(rule, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, " ")
}
