package main

import "testing"

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
