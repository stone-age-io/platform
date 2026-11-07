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

// The feed's invariant: an entry is visible to exactly those who could read the
// record it describes. The feed's read rule is org-scoped with no role branch,
// so every covered collection's must be too. This fails if one has a role
// clause, e.g. if `memberships` is added. The regression is one line and has no
// symptom.
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

// The paired "can": the collections left out really are role-scoped. Without
// this, flattening every rule would pass the test above.
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
	// The fallback order keeps a feed line readable for thing_type_operations,
	// which has a name and no code.
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

// stripRuleComments removes the // lines so a rule's prose, which often
// mentions roles, cannot satisfy a check on its logic.
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

// RESOURCE_ROUTES in ui/src/views/activity/ActivityView.vue maps each feed noun
// to a console route, and is a hand-copied set of activityCollections' values.
// This keeps the two in step in both directions: a noun with no route silently
// offers no way to open the record, and a route for a noun the server never
// writes is dead code.
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

// The object literal's body, from the opening brace to the closing brace at
// column zero. Anchored on the declaration because `'thing': {` alone is not
// distinctive.
//
// `\r?\n`, not `\n`: .gitattributes does not pin *.vue to LF, so a Windows
// worktree has it as CRLF and a bare `\n` would fail locally and pass in CI. Any
// test that reads a UI source file needs the same tolerance.
var resourceRoutesRe = regexp.MustCompile(`(?s)const RESOURCE_ROUTES\b[^\n]*\{\r?\n(.*?)\r?\n\}`)

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
