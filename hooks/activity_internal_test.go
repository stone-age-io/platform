package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The invariant the whole feature rests on:
//
//	an activity entry is visible to exactly those who could read the record
//	it describes.
//
// The feed's own read rule is org-scoped with no role branch, so it may only
// describe records whose OWN read rule is org-scoped with no role branch. This
// reads schema.json and fails if any covered collection has a role clause --
// which is what stops someone adding `memberships` for a richer feed and
// silently telling every member whose role changed.
//
// The same shape and the same reason as audit_snapshots_test.go: the regression
// is one line, and it has no symptom.
func TestActivityCollectionsAreOrgReadable(t *testing.T) {
	collections := loadSchemaCollections(t)

	covered := ActivityCollectionNames()
	if len(covered) == 0 {
		t.Fatal("the activity collection list is empty; this test would pass vacuously")
	}

	for _, name := range covered {
		rule, ok := collections[name]
		if !ok {
			t.Errorf("%q is in the activity feed but not in schema.json", name)
			continue
		}
		if rule == nil {
			t.Errorf("%q has a nil listRule (superuser only), so the feed would describe records no tenant can read", name)
			continue
		}

		body := stripRuleComments(*rule)

		if strings.Contains(body, "role ?=") || strings.Contains(body, "role =") {
			t.Errorf("%s.listRule has a ROLE branch, so a reader of the feed could see entries for "+
				"records they cannot read; remove it from the activity feed or give the feed an audience tier", name)
		}
		if !strings.Contains(body, "organization = @request.auth.current_organization") {
			t.Errorf("%s.listRule is not org-scoped in the expected form; the feed assumes it is", name)
		}
	}
}

// The paired "can": the collections deliberately left OUT really are the ones a
// shared feed would leak. Without this, emptying the list or flattening every
// rule would pass the test above while destroying the property.
func TestActivityExclusionsAreRoleScoped(t *testing.T) {
	collections := loadSchemaCollections(t)

	covered := map[string]bool{}
	for _, n := range ActivityCollectionNames() {
		covered[n] = true
	}

	for _, name := range []string{"memberships", "invites", "nats_roles", "nebula_networks"} {
		if covered[name] {
			t.Errorf("%q is in the activity feed, but its own reads stop at owner/admin", name)
		}

		rule, ok := collections[name]
		if !ok || rule == nil {
			t.Errorf("%s: expected a listRule in schema.json", name)
			continue
		}
		body := stripRuleComments(*rule)
		if !strings.Contains(body, "role ?=") && !strings.Contains(body, "organization.owner") {
			t.Errorf("%s.listRule no longer looks role-scoped -- if its reads opened up, this "+
				"guard no longer describes the risk it was written for", name)
		}
	}
}

func TestRecordLabelPrefersNameThenCode(t *testing.T) {
	// Covered without a database: recordLabel only reads fields off a record.
	// The fallback order is what keeps a feed line readable for
	// thing_type_operations, which has a name and no code.
	cases := []struct {
		what, name, code, id, want string
	}{
		{"name wins", "Front Door", "DOOR-1", "abc", "Front Door"},
		{"code when unnamed", "", "DOOR-1", "abc", "DOOR-1"},
		{"id when neither", "", "", "abc", "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			got := firstNonEmpty(tc.name, tc.code, tc.id)
			if got != tc.want {
				t.Errorf("label = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- helpers ---

func loadSchemaCollections(t *testing.T) map[string]*string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "schema.json"))
	if err != nil {
		t.Fatalf("read schema.json: %v", err)
	}

	var parsed []struct {
		Name     string  `json:"name"`
		ListRule *string `json:"listRule"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse schema.json: %v", err)
	}

	out := make(map[string]*string, len(parsed))
	for _, c := range parsed {
		out[c.Name] = c.ListRule
	}
	return out
}

// stripRuleComments removes the // lines so a rule's prose cannot satisfy a
// check on its logic -- every rule in this schema carries a comment mentioning
// roles.
func stripRuleComments(rule string) string {
	var b strings.Builder
	for _, line := range strings.Split(rule, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// The console's "Open record" button resolves a feed entry to a route by the
// product NOUN this file snapshots into `resource` -- RESOURCE_ROUTES in
// ui/src/views/activity/ActivityView.vue. That map is a hand-copied set of the
// values below, so it can drift, and drift here is silent: adding a sixth
// collection to activityCollections gives its entries a feed row with no way to
// reach the record, and nothing errors at either end.
//
// Same trick, and the same reason, as managed_org_exports_test.go: the copy is
// fine, the copy going stale is not. The direction that matters is
// Go -> TypeScript (a noun with no route), but a route keyed on a noun the
// server never writes is also worth catching -- it is dead code that reads like
// coverage.
func TestActivityNounsAllHaveAConsoleRoute(t *testing.T) {
	dir, ok := callerDir(t)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	path := filepath.Join(dir, "..", "ui", "src", "views", "activity", "ActivityView.vue")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	block := resourceRoutesRe.FindSubmatch(raw)
	if block == nil {
		t.Fatalf("RESOURCE_ROUTES not found in %s.\n"+
			"If it moved or was reformatted, update resourceRoutesRe -- do NOT delete this test: it is the "+
			"only thing tying the feed's nouns to the console routes that resolve them.", path)
	}

	routed := map[string]bool{}
	for _, m := range routeKeyRe.FindAllSubmatch(block[1], -1) {
		routed[string(m[1])] = true
	}
	if len(routed) == 0 {
		t.Fatal("parsed RESOURCE_ROUTES but found no keys; this test would pass vacuously")
	}

	for collection, noun := range activityCollections {
		if !routed[noun] {
			t.Errorf("collection %q reports itself as %q, which RESOURCE_ROUTES does not resolve; "+
				"a feed entry for it offers no way to open the record", collection, noun)
		}
		delete(routed, noun)
	}

	for noun := range routed {
		t.Errorf("RESOURCE_ROUTES resolves %q, which no collection in activityCollections reports; "+
			"the server never writes that noun, so the branch is dead", noun)
	}
}

// The object literal's body, from the opening brace to the line that closes it
// at column zero. Deliberately anchored on the declaration rather than matching
// keys across the whole file: `'thing': {` is not a distinctive enough shape to
// grep for on its own.
var resourceRoutesRe = regexp.MustCompile(`(?s)const RESOURCE_ROUTES\b[^\n]*\{\n(.*?)\n\}`)

// One quoted key per entry.
var routeKeyRe = regexp.MustCompile(`(?m)^\s*'([^']+)':\s*\{`)

// callerDir returns the directory holding this test file.
func callerDir(t *testing.T) (string, bool) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	return filepath.Dir(thisFile), true
}
