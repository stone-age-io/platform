package main

import (
	"os"
	"strings"
	"testing"
)

// orgCodeFor keeps `bootstrap` and migrations/schema_update_org_code.go
// assigning the same code to the same org. hooks.TestSlugify covers the
// derivation; this covers the fallback.
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

// The documented default names derive to the fallback codes, so a default
// install gets the same codes whether derived or fallen back.
func TestDefaultInfrastructureNamesDeriveToTheOldPinnedCodes(t *testing.T) {
	if got := orgCodeFor("System", systemOrgCode); got != systemOrgCode {
		t.Errorf("system org: got %q, want %q", got, systemOrgCode)
	}
	if got := orgCodeFor("Operator", operatorOrgCode); got != operatorOrgCode {
		t.Errorf("operator org: got %q, want %q", got, operatorOrgCode)
	}
}

// hooks/relation_tenancy.go re-checks every relation when a record's
// organization moves, which adopting the $SYS records does. So bootstrap must
// link the NATS account and role before the user that points at them, or the
// adoption is refused. Neither the Go suite nor scripts/test-authz.sh runs
// bootstrap, so this asserts the order in the source.
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
