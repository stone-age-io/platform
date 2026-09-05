package hooks_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// Two organizations, each with the NATS account the provisioning hook mints for
// it. Org B is the tenant nothing in org A may point at.
type fixture struct {
	app      core.App
	orgA     *core.Record
	orgB     *core.Record
	accountA *core.Record
	accountB *core.Record
	roleA    *core.Record
	owner    *core.Record
}

// One app for the whole file. Standing one up costs the better part of ten
// seconds — four libraries bootstrap and every migration runs — and the subtests
// below only need records that do not collide, not isolation.
func setup(t *testing.T) fixture {
	t.Helper()
	app := testutil.SetupApp(t)

	save := func(what string, rec *core.Record) *core.Record {
		if err := app.Save(rec); err != nil {
			t.Fatalf("create %s: %v", what, err)
		}
		return rec
	}

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}

	// organizations.owner is required, so an org cannot be created without one.
	owner := newRecord("users")
	owner.Set("email", "owner@example.test")
	owner.Set("password", "owner-password-123")
	save("owner user", owner)

	newOrg := func(name string) *core.Record {
		rec := newRecord("organizations")
		rec.Set("name", name)
		rec.Set("active", true)
		rec.Set("owner", owner.Id)
		return save("org "+name, rec)
	}

	accountFor := func(org *core.Record) *core.Record {
		rec, err := app.FindFirstRecordByFilter("nats_accounts",
			"organization = {:org}", map[string]any{"org": org.Id})
		if err != nil {
			t.Fatalf("no NATS account provisioned for %s: %v", org.GetString("name"), err)
		}
		return rec
	}

	orgA := newOrg("Alpha Industries")
	orgB := newOrg("Beta Logistics")

	// A role inside org A, so that in the cross-org case below the account is the
	// only thing wrong and a rejection cannot be blamed on the role.
	roleA := newRecord("nats_roles")
	roleA.Set("name", "device")
	roleA.Set("organization", orgA.Id)
	save("role in org A", roleA)

	return fixture{
		app:      app,
		orgA:     orgA,
		orgB:     orgB,
		accountA: accountFor(orgA),
		accountB: accountFor(orgB),
		roleA:    roleA,
		owner:    owner,
	}
}

func (f fixture) newNatsUser(t *testing.T, name string, account *core.Record) *core.Record {
	t.Helper()
	col, err := f.app.FindCollectionByNameOrId("nats_users")
	if err != nil {
		t.Fatalf("nats_users collection: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("nats_username", name)
	rec.Set("email", name+"@example.test")
	rec.Set("password", "probe-password-123")
	rec.Set("organization", f.orgA.Id)
	rec.Set("account_id", account.Id)
	rec.Set("role_id", f.roleA.Id)
	rec.Set("active", true)
	return rec
}

func (f fixture) newLocation(t *testing.T, name string, org *core.Record) *core.Record {
	t.Helper()
	col, err := f.app.FindCollectionByNameOrId("locations")
	if err != nil {
		t.Fatalf("locations collection: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("name", name)
	rec.Set("organization", org.Id)
	return rec
}

func TestRelationTenancy(t *testing.T) {
	f := setup(t)

	// The bug this guard exists for. pb-nats signs a user JWT with whatever
	// account_id names (internal/sync/manager.go, generateUserJWT), so a record in
	// org A naming org B's account yields a working credential inside org B.
	t.Run("a NATS user cannot borrow another org's account", func(t *testing.T) {
		err := f.app.Save(f.newNatsUser(t, "borrower", f.accountB))
		if err == nil {
			t.Fatal("saved a NATS user in org A pointing at org B's account; expected a rejection")
		}
		if !strings.Contains(err.Error(), "account_id") {
			t.Errorf("the error should name the offending field, got: %v", err)
		}
	})

	// The paired "can". Without it a blanket refusal would pass the test above.
	t.Run("a NATS user accepts its own org's account", func(t *testing.T) {
		if err := f.app.Save(f.newNatsUser(t, "resident", f.accountA)); err != nil {
			t.Fatalf("a NATS user naming its own org's account must save: %v", err)
		}
	})

	// The same hole through a different door: nats_users.updateRule freezes
	// `organization` and says nothing about account_id.
	t.Run("a NATS user cannot be repointed at another org's account", func(t *testing.T) {
		rec := f.newNatsUser(t, "repointed", f.accountA)
		if err := f.app.Save(rec); err != nil {
			t.Fatalf("setup save: %v", err)
		}

		rec.Set("account_id", f.accountB.Id)
		if err := f.app.Save(rec); err == nil {
			t.Fatal("repointed a NATS user at another org's account; expected a rejection")
		}
	})

	// The guard is derived from the schema rather than a list of fields somebody
	// remembered, so a relation with no NATS involvement is covered the same way.
	t.Run("a location cannot be parented into another org", func(t *testing.T) {
		parentInB := f.newLocation(t, "Beta HQ", f.orgB)
		if err := f.app.Save(parentInB); err != nil {
			t.Fatalf("create parent in org B: %v", err)
		}

		child := f.newLocation(t, "Alpha Floor 1", f.orgA)
		child.Set("parent", parentInB.Id)
		if err := f.app.Save(child); err == nil {
			t.Fatal("parented an org A location under an org B location; expected a rejection")
		}

		// Same write, same-org parent: must succeed, or the assertion above proves
		// only that the save failed for some reason.
		parentInA := f.newLocation(t, "Alpha HQ", f.orgA)
		if err := f.app.Save(parentInA); err != nil {
			t.Fatalf("create parent in org A: %v", err)
		}
		child.Set("parent", parentInA.Id)
		if err := f.app.Save(child); err != nil {
			t.Fatalf("a same-org parent must be accepted: %v", err)
		}
	})

	// A relation whose target has no `organization` column is none of this
	// guard's business. memberships.user points at `users`, which is global by
	// design, and refusing it would make membership creation impossible.
	t.Run("relations to unowned collections are left alone", func(t *testing.T) {
		// Not f.owner: creating an org already gave them a membership in it, and
		// memberships are unique per (organization, user).
		userCol, err := f.app.FindCollectionByNameOrId("users")
		if err != nil {
			t.Fatalf("users collection: %v", err)
		}
		joiner := core.NewRecord(userCol)
		joiner.Set("email", "joiner@example.test")
		joiner.Set("password", "joiner-password-123")
		if err := f.app.Save(joiner); err != nil {
			t.Fatalf("create joining user: %v", err)
		}

		col, err := f.app.FindCollectionByNameOrId("memberships")
		if err != nil {
			t.Fatalf("memberships collection: %v", err)
		}
		membership := core.NewRecord(col)
		membership.Set("user", joiner.Id)
		membership.Set("organization", f.orgA.Id)
		membership.Set("role", "member")

		if err := f.app.Save(membership); err != nil {
			t.Fatalf("a membership pointing at a global user must save: %v", err)
		}
	})
}
