package main

import (
	"os"
	"strings"
	"testing"
)

// orgCodeFor is what keeps `bootstrap` and the backfill in
// migrations/schema_update_org_code.go assigning the same code to the same
// organization. The fallback branch is the only part not already covered by
// hooks.TestSlugify, and it is the part that used to be the whole behaviour.
func TestOrgCodeFor(t *testing.T) {
	cases := []struct {
		what     string
		name     string
		fallback string
		want     string
	}{
		{"a usable name wins over the fallback", "System", systemOrgCode, "system"},
		{"a leading digit is a code now, not a fallback", "816tech", operatorOrgCode, "816tech"},
		{"multi-word names slugify", "Acme Industries", operatorOrgCode, "acme-industries"},
		{"a name with no alphanumerics falls back", "!!!", operatorOrgCode, "operator"},
		{"a single-character name is below the minimum", "X", systemOrgCode, "system"},
		{"an empty name falls back", "", operatorOrgCode, "operator"},
	}

	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			if got := orgCodeFor(tc.name, tc.fallback); got != tc.want {
				t.Errorf("orgCodeFor(%q, %q) = %q, want %q", tc.name, tc.fallback, got, tc.want)
			}
		})
	}
}

// The names the two infrastructure orgs are documented with derive to the codes
// bootstrap used to pin, so the default install is unchanged by the switch to
// deriving. Only a deployment that renamed them sees a different code -- which
// is the point, since the migration would have derived one from the name too.
func TestDefaultInfrastructureNamesDeriveToTheOldPinnedCodes(t *testing.T) {
	if got := orgCodeFor("System", systemOrgCode); got != systemOrgCode {
		t.Errorf("system org: got %q, want %q", got, systemOrgCode)
	}
	if got := orgCodeFor("Operator", operatorOrgCode); got != operatorOrgCode {
		t.Errorf("operator org: got %q, want %q", got, operatorOrgCode)
	}
}

// The `bootstrap` command had NO test coverage until this one, and it cost a
// shipped regression: v0.7.0 tightened hooks/relation_tenancy.go to re-check
// every relation when a record's own organization moves, which is precisely
// what adopting the pre-seeded $SYS records does. Linking the NATS user while
// its role was still unlinked compared a user now in the System org against a
// role still blank, and bootstrap died with "the role_id field must reference a
// record in the same organization" -- leaving an install with no working NATS
// identity and no operator context.
//
// Nothing caught it: the full Go suite never runs this command, and
// scripts/test-authz.sh stands its server up with `superuser upsert` +
// `migrate up` + `serve` and skips bootstrap entirely.
//
// This asserts the ORDER rather than the outcome, because the order is the
// fix: link what is pointed AT before what points at it. An outcome test
// against a real database would be a second harness; the ordering is the thing
// that can silently regress when someone tidies the block.
func TestBootstrapLinksNatsTargetsBeforeTheUserThatPointsAtThem(t *testing.T) {
	source, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatalf("read bootstrap.go: %v", err)
	}

	body := string(source)
	roleAt := strings.Index(body, `natsOpts.RoleCollectionName, org.Id, "NATS Role"`)
	userAt := strings.Index(body, `natsOpts.UserCollectionName, org.Id, "NATS User"`)
	accountAt := strings.Index(body, `natsOpts.AccountCollectionName, org.Id, "NATS Account"`)

	if roleAt < 0 || userAt < 0 || accountAt < 0 {
		t.Fatal("could not find the linkSingleton calls; this guard no longer describes bootstrap")
	}

	if accountAt > userAt {
		t.Error("the NATS account is linked after the user that points at it via account_id; " +
			"relation tenancy will refuse the adoption")
	}
	if roleAt > userAt {
		t.Error("the NATS role is linked after the user that points at it via role_id; " +
			"relation tenancy will refuse the adoption with " +
			`"the role_id field must reference a record in the same organization"`)
	}
}
