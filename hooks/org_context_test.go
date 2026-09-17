package hooks_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"platform/hooks"
	"platform/internal/testutil"
)

// requireMembership is the one guard standing between a route that writes with
// app.Save() -- bypassing every API rule -- and another tenant's data. Every
// route using it is covered end to end by scripts/test-authz.sh, but only for
// the routes that exist today; the reason the helper is shared at all is the
// route someone writes next. These assert its refusals directly.
//
// The two that matter most are the ones this codebase has actually shipped as
// bugs: a blank active organization (the empty-string tenancy collapse, where
// '' = '' compared equal and read across tenants) and a membership that does
// not correlate to the claimed organization.

type orgCtxFixture struct {
	app    core.App
	orgA   *core.Record
	orgB   *core.Record
	owner  *core.Record
	member *core.Record
	viewer *core.Record
	outsdr *core.Record // a member of org B whose context claims org A
	thing  *core.Record
}

func setupOrgCtx(t *testing.T) orgCtxFixture {
	t.Helper()
	app := testutil.SetupApp(t)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}
	save := func(what string, rec *core.Record) *core.Record {
		if err := app.Save(rec); err != nil {
			t.Fatalf("create %s: %v", what, err)
		}
		return rec
	}

	newUser := func(email string) *core.Record {
		rec := newRecord("users")
		rec.Set("email", email)
		rec.Set("password", "probe-password-123")
		return save("user "+email, rec)
	}

	// organizations.owner is required, and creating one mints the owner's
	// membership through hooks.RegisterOrgMembership.
	owner := newUser("owner@example.test")

	newOrg := func(name string) *core.Record {
		rec := newRecord("organizations")
		rec.Set("name", name)
		rec.Set("active", true)
		rec.Set("owner", owner.Id)
		return save("org "+name, rec)
	}
	orgA := newOrg("Alpha Industries")
	orgB := newOrg("Beta Logistics")

	joinAs := func(email, role string, org *core.Record) *core.Record {
		u := newUser(email)
		m := newRecord("memberships")
		m.Set("user", u.Id)
		m.Set("organization", org.Id)
		m.Set("role", role)
		save("membership "+role, m)
		u.Set("current_organization", org.Id)
		return save("user context "+email, u)
	}

	member := joinAs("member@example.test", "member", orgA)
	viewer := joinAs("viewer@example.test", "viewer", orgA)

	// The cross-tenant probe: a genuine member of org B, but whose active
	// organization claims org A. Only the correlated membership lookup catches
	// this -- the blank check does not, because neither side is blank.
	outsdr := joinAs("outsider@example.test", "owner", orgB)
	outsdr.Set("current_organization", orgA.Id)
	save("outsider pointed at org A", outsdr)

	// owner's context is org A. It was set by the membership hook for whichever
	// org was created last, so set it explicitly rather than relying on order.
	owner.Set("current_organization", orgA.Id)
	save("owner context", owner)

	thing := newRecord("things")
	thing.Set("organization", orgA.Id)
	thing.Set("name", "Door Controller")
	thing.Set("code", "DOOR-1")
	thing.Set("email", "door-1@alpha.thing.local")
	thing.Set("emailVisibility", true)
	thing.Set("password", "thing-password-123")
	thing.Set("active", true)
	save("thing", thing)

	return orgCtxFixture{
		app: app, orgA: orgA, orgB: orgB,
		owner: owner, member: member, viewer: viewer, outsdr: outsdr, thing: thing,
	}
}

// call runs the guard as a route would, with only the authenticated record to
// go on -- there is deliberately no way to pass an organization in.
func (f orgCtxFixture) call(auth *core.Record, roles []string) (string, string, error) {
	re := &core.RequestEvent{App: f.app, Auth: auth}
	return hooks.RequireMembership(re, "memberships", roles)
}

func status(err error) int {
	if err == nil {
		return 200
	}
	return router.ToApiError(err).Status
}

func TestRequireMembershipAllowsTheRolesItNames(t *testing.T) {
	f := setupOrgCtx(t)

	// Paired allow/deny on the SAME caller, for the same reason every check in
	// test-authz.sh is paired: a guard that refused everything would pass a
	// deny-only test.
	t.Run("owner passes the manager gate", func(t *testing.T) {
		org, role, err := f.call(f.owner, hooks.RolesOrgManager)
		if err != nil {
			t.Fatalf("owner refused: %v", err)
		}
		if org != f.orgA.Id {
			t.Errorf("organization = %q, want %q", org, f.orgA.Id)
		}
		if role != "owner" {
			t.Errorf("role = %q, want owner", role)
		}
	})

	t.Run("member passes the inventory gate", func(t *testing.T) {
		org, role, err := f.call(f.member, hooks.RolesInventory)
		if err != nil {
			t.Fatalf("member refused: %v", err)
		}
		if org != f.orgA.Id || role != "member" {
			t.Errorf("got (%q, %q), want (%q, member)", org, role, f.orgA.Id)
		}
	})

	t.Run("member is refused the manager gate", func(t *testing.T) {
		if _, _, err := f.call(f.member, hooks.RolesOrgManager); status(err) != 403 {
			t.Errorf("status = %d, want 403", status(err))
		}
	})

	// viewer holds read capability and no write branch anywhere. It is the
	// probe for the inventory gate the way `dashboard` is for the authz suite.
	t.Run("viewer is refused the inventory gate", func(t *testing.T) {
		if _, _, err := f.call(f.viewer, hooks.RolesInventory); status(err) != 403 {
			t.Errorf("status = %d, want 403", status(err))
		}
	})
}

func TestRequireMembershipRefusesABlankOrganization(t *testing.T) {
	f := setupOrgCtx(t)

	// hooks/membership_lifecycle.go blanks current_organization on the way out
	// of an organization, so this is an ordinary product state and not a
	// contrived one. Left unguarded it is the empty-string tenancy collapse.
	stranded := f.member
	stranded.Set("current_organization", "")

	if _, _, err := f.call(stranded, hooks.RolesInventory); status(err) != 400 {
		t.Errorf("status = %d, want 400 for a blank active organization", status(err))
	}
}

func TestRequireMembershipRefusesAnUncorrelatedMembership(t *testing.T) {
	f := setupOrgCtx(t)

	// An owner of org B, claiming org A. Neither side is blank, so only the
	// correlated lookup refuses this -- and it is the check that makes the
	// organization come from the membership rather than from the claim.
	org, _, err := f.call(f.outsdr, hooks.RolesOrgManager)
	if status(err) != 403 {
		t.Fatalf("status = %d, want 403; an owner of another organization reached org A", status(err))
	}
	if org != "" {
		t.Errorf("organization = %q, want empty on refusal", org)
	}

	// Paired allow: the same caller IS an owner, in their own organization.
	f.outsdr.Set("current_organization", f.orgB.Id)
	if _, role, err := f.call(f.outsdr, hooks.RolesOrgManager); err != nil {
		t.Errorf("refused in their own organization: %v", err)
	} else if role != "owner" {
		t.Errorf("role = %q, want owner", role)
	}
}

func TestRequireMembershipRefusesNonUserCallers(t *testing.T) {
	f := setupOrgCtx(t)

	// A Thing authenticates against `things` and has no membership at all. The
	// route binding already restricts this; the guard restates it so a change
	// to the Bind cannot silently widen who reaches the body.
	if _, _, err := f.call(f.thing, hooks.RolesInventory); status(err) != 401 {
		t.Errorf("thing: status = %d, want 401", status(err))
	}
	if _, _, err := f.call(nil, hooks.RolesInventory); status(err) != 401 {
		t.Errorf("unauthenticated: status = %d, want 401", status(err))
	}
}

func TestRequireMembershipRefusesAnEmptyRoleList(t *testing.T) {
	f := setupOrgCtx(t)

	// The footgun this signature exists to close. An empty role list would
	// build a filter with no role clause and match ANY membership, so a caller
	// that lost its roles in a refactor would silently widen to every role
	// rather than fail. It is a programming error, so it is a 500, and the
	// caller is a genuine owner to prove the refusal is about the roles.
	if _, _, err := f.call(f.owner, nil); status(err) != 500 {
		t.Errorf("status = %d, want 500 for an empty role list", status(err))
	}
	if _, _, err := f.call(f.owner, []string{}); status(err) != 500 {
		t.Errorf("status = %d, want 500 for an empty role slice", status(err))
	}
}
