package demoseed_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/demoseed"
	"platform/internal/testutil"
)

// A modest fleet keeps the suite quick; every property asserted below is
// independent of scale.
const testThings = 60

// One seeded app, shared by every read-only test in this file.
//
// Standing up an app costs several seconds — four libraries bootstrap their
// collections, every migration runs — and seeding costs several more, mostly in
// key generation: three NATS accounts, three Nebula CAs, and a signed identity
// for every Thing. Paying that per test took the package from seconds to minutes
// and would have made `go test ./...` something people skip.
//
// The two tests that MUTATE (idempotency, and topping up the fleet) build their
// own app, because sharing one would make them order-dependent.
var shared core.App

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "demoseed")
	if err != nil {
		fmt.Fprintln(os.Stderr, "temp dir:", err)
		os.Exit(1)
	}

	app, err := testutil.NewApp(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "harness:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	if _, err := demoseed.Run(app, demoseed.Options{Things: testThings}); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		_ = app.ResetBootstrapState()
		os.RemoveAll(dir)
		os.Exit(1)
	}
	shared = app

	code := m.Run()

	_ = app.ResetBootstrapState()
	os.RemoveAll(dir)
	os.Exit(code)
}

func count(t *testing.T, app core.App, collection string) int {
	t.Helper()
	recs, err := app.FindAllRecords(collection)
	if err != nil {
		t.Fatalf("count %s: %v", collection, err)
	}
	return len(recs)
}

func TestSeedPopulatesEveryCollectionItClaimsTo(t *testing.T) {
	app := shared

	for _, c := range []struct {
		name string
		min  int
	}{
		{"organizations", 3},
		{"users", 11},
		{"memberships", 13},
		{"location_types", 13},
		{"locations", 25},
		{"message_schemas", 21},
		{"thing_type_operations", 35},
		{"thing_types", 22},
		{"things", testThings},
		{"leaf_nodes", 5},
		{"nats_accounts", 3},
		{"nats_roles", 15},
		{"nebula_ca", 3},
		{"nebula_networks", 3},
		{"nebula_hosts", 3},
	} {
		if got := count(t, app, c.name); got < c.min {
			t.Errorf("%s: got %d records, want at least %d", c.name, got, c.min)
		}
	}
}

// The whole seed is built on organizations.code, and a code is immutable once
// written — so a run that produced the wrong one could not be repaired in place.
func TestOrganizationsCarryTheCodesTheHelpdeskDemoJoinsOn(t *testing.T) {
	app := shared

	for _, code := range []string{"northwind", "ironbridge", "galewind"} {
		org, err := app.FindFirstRecordByFilter("organizations", "code = {:c}", dbx.Params{"c": code})
		if err != nil || org == nil {
			t.Fatalf("no organization with code %q", code)
		}
		if org.GetString("owner") == "" {
			t.Errorf("organization %q has no owner", code)
		}
	}
}

// Every organization must come out of the provisioning hook with its NATS
// account and Nebula CA. If this fails, the seeder was wired into a binary that
// does not register the hook — which would produce a demo that looks complete in
// the inventory screens and has no credentials behind any of it.
func TestEveryOrganizationIsProvisioned(t *testing.T) {
	app := shared

	orgs, err := app.FindAllRecords("organizations")
	if err != nil {
		t.Fatal(err)
	}
	for _, org := range orgs {
		acct, err := app.FindFirstRecordByFilter("nats_accounts",
			"organization = {:o}", dbx.Params{"o": org.Id})
		if err != nil || acct == nil {
			t.Errorf("organization %q has no NATS account", org.GetString("code"))
			continue
		}
		if acct.GetString("public_key") == "" {
			t.Errorf("NATS account for %q has no public key", org.GetString("code"))
		}
		ca, err := app.FindFirstRecordByFilter("nebula_ca",
			"organization = {:o}", dbx.Params{"o": org.Id})
		if err != nil || ca == nil {
			t.Errorf("organization %q has no Nebula CA", org.GetString("code"))
		}
	}
}

// A leaf node's NATS user is minted by RegisterLeafNodeProvisioning, not by the
// seeder. This is what proves the seed goes in through the platform's own
// provisioning path rather than around it.
func TestLeafNodesGetTheirNatsUserFromTheHook(t *testing.T) {
	app := shared

	leaves, err := app.FindAllRecords("leaf_nodes")
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves) == 0 {
		t.Fatal("no leaf nodes seeded")
	}
	for _, leaf := range leaves {
		if leaf.GetString("nats_user") == "" {
			t.Errorf("leaf node %q has no NATS user", leaf.GetString("code"))
		}
		if got, want := leaf.GetString("domain"), "edge-"+leaf.GetString("code"); got != want {
			t.Errorf("leaf node %q domain = %q, want %q", leaf.GetString("code"), got, want)
		}
	}
}

// things.authRule is `active = true`, and a PocketBase bool has no schema
// default. A Thing seeded without the flag set would be locked out of the API —
// the exact bug POST /api/org/things was written to fix.
func TestEveryActiveThingCanActuallyAuthenticate(t *testing.T) {
	app := shared

	all, err := app.FindAllRecords("things")
	if err != nil {
		t.Fatal(err)
	}
	var active, inactive int
	for _, thing := range all {
		if thing.GetBool("active") {
			active++
		} else {
			inactive++
		}
		if thing.GetString("nats_user") == "" {
			t.Errorf("thing %q has no NATS identity", thing.GetString("code"))
		}
		if thing.GetString("organization") == "" {
			t.Errorf("thing %q has no organization", thing.GetString("code"))
		}
	}
	if active == 0 {
		t.Error("no active things — every seeded device would be locked out by things.authRule")
	}
	// The demo deliberately includes decommissioned devices so the inactive
	// badge and stone_age_inactive_records have something to show.
	if inactive == 0 {
		t.Error("no inactive things — the decommissioned state is unrepresented")
	}
}

// A Thing's real capability is its NATS credential. Deactivating one has to
// reach nats_users, or it has only closed the console door.
func TestDecommissionedThingsHaveTheirCredentialRevoked(t *testing.T) {
	app := shared

	inactive, err := app.FindAllRecords("things", dbx.NewExp("active = false"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inactive) == 0 {
		t.Skip("no inactive things to check")
	}
	for _, thing := range inactive {
		u, err := app.FindRecordById("nats_users", thing.GetString("nats_user"))
		if err != nil {
			t.Errorf("thing %q: %v", thing.GetString("code"), err)
			continue
		}
		if u.GetBool("active") {
			t.Errorf("thing %q is decommissioned but its NATS identity is still active",
				thing.GetString("code"))
		}
	}
}

// Every one of the five console roles should be reachable by logging in, or the
// demo cannot show what they differ on.
func TestAllFiveConsoleRolesAreRepresented(t *testing.T) {
	app := shared

	seen := map[string]bool{}
	memberships, err := app.FindAllRecords("memberships")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range memberships {
		seen[m.GetString("role")] = true
		// A console role is not a NATS role, and the pairing is what this demo
		// is meant to teach. A membership with no NATS identity teaches nothing.
		if m.GetString("nats_user") == "" {
			t.Errorf("membership %s has no NATS identity linked", m.Id)
		}
	}
	for _, role := range []string{"owner", "admin", "member", "viewer", "dashboard"} {
		if !seen[role] {
			t.Errorf("no membership holds the %q role", role)
		}
	}
}

// The read-only console roles must be paired with a NATS role that is genuinely
// read-only. Otherwise the demo ships the trap it exists to illustrate: an
// auditor who is read-only in PocketBase and can publish anywhere on the bus.
func TestReadOnlyConsoleRolesGetAReadOnlyNatsRole(t *testing.T) {
	app := shared

	memberships, err := app.FindAllRecords("memberships")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, m := range memberships {
		role := m.GetString("role")
		if role != "viewer" && role != "dashboard" {
			continue
		}
		u, err := app.FindRecordById("nats_users", m.GetString("nats_user"))
		if err != nil {
			t.Fatalf("membership %s: %v", m.Id, err)
		}
		natsRole, err := app.FindRecordById("nats_roles", u.GetString("role_id"))
		if err != nil {
			t.Fatalf("nats user %s: %v", u.Id, err)
		}
		if got := natsRole.GetString("name"); got != "console-readonly" {
			t.Errorf("console role %q is paired with NATS role %q, want console-readonly",
				role, got)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no viewer or dashboard memberships to check")
	}
}

// The read-only NATS role must not carry a publish wildcard. This is the actual
// property the pairing above is trying to buy.
func TestConsoleReadonlyRoleCannotPublishAnywhere(t *testing.T) {
	app := shared

	roles, err := app.FindAllRecords("nats_roles", dbx.NewExp("name = 'console-readonly'"))
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) == 0 {
		t.Fatal("console-readonly role was not seeded")
	}
	for _, r := range roles {
		var perms []string
		if err := r.UnmarshalJSONField("publish_permissions", &perms); err != nil {
			t.Fatalf("publish_permissions: %v", err)
		}
		for _, p := range perms {
			if p == ">" || p == "*" {
				t.Errorf("console-readonly publishes %q — that is not read-only", p)
			}
		}
	}
}

// Re-running must converge, not duplicate. A demo instance someone re-seeds
// after poking at it should end up where it started.
func TestSeedingTwiceChangesNothing(t *testing.T) {
	app := testutil.SetupApp(t)

	first, err := demoseed.Run(app, demoseed.Options{Things: testThings})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	before := map[string]int{}
	for _, c := range []string{"organizations", "things", "locations", "nats_users", "memberships", "leaf_nodes"} {
		before[c] = count(t, app, c)
	}

	second, err := demoseed.Run(app, demoseed.Options{Things: testThings})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	for c, n := range before {
		if got := count(t, app, c); got != n {
			t.Errorf("%s: %d records after one run, %d after two", c, n, got)
		}
	}
	if len(first.Created) == 0 {
		t.Error("first run created nothing")
	}
	if len(second.Created) != 0 {
		t.Errorf("second run created %v, want nothing", second.Created)
	}
}

// Raising --things on an existing instance should add to the fleet rather than
// collide on a code that is already taken.
func TestRaisingTheThingCountTopsUp(t *testing.T) {
	app := testutil.SetupApp(t)

	if _, err := demoseed.Run(app, demoseed.Options{Things: 40}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstCount := count(t, app, "things")

	if _, err := demoseed.Run(app, demoseed.Options{Things: 70}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	secondCount := count(t, app, "things")

	if secondCount <= firstCount {
		t.Errorf("things did not grow: %d then %d", firstCount, secondCount)
	}
	if secondCount != 70 {
		t.Errorf("converged on %d things, want 70", secondCount)
	}
}

// Codes are unique within an organization, which is what the partial indexes
// enforce and what every cross-app join depends on.
func TestCodesAreUniqueWithinAnOrganization(t *testing.T) {
	app := shared

	for _, collection := range []string{"things", "locations", "thing_types", "location_types", "leaf_nodes"} {
		recs, err := app.FindAllRecords(collection)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, r := range recs {
			code := r.GetString("code")
			if code == "" {
				continue
			}
			key := r.GetString("organization") + ":" + code
			if seen[key] {
				t.Errorf("%s: duplicate code %q within one organization", collection, code)
			}
			seen[key] = true
		}
	}
}

// Things span more than sensors. The platform's claim is that an application is
// a first-class participant with its own signed identity; a demo of nothing but
// probes quietly contradicts it.
func TestThingsSpanDevicesGatewaysAndApplications(t *testing.T) {
	app := shared

	types, err := app.FindAllRecords("thing_types")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, tt := range types {
		byID[tt.Id] = tt.GetString("code")
	}

	all, err := app.FindAllRecords("things")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, thing := range all {
		switch byID[thing.GetString("type")] {
		case "wms-connector", "coldchain-rules", "oee-analytics", "mes-connector", "scada-bridge", "market-feed":
			kinds["application"] = true
		case "edge-gateway", "access-controller":
			kinds["gateway"] = true
		case "dock-display", "ops-wallboard":
			kinds["appliance"] = true
		default:
			kinds["device"] = true
		}
	}
	for _, want := range []string{"device", "gateway", "application", "appliance"} {
		if !kinds[want] {
			t.Errorf("no thing of kind %q was seeded", want)
		}
	}
}

// The stone-access join.
//
// Every code below is ALSO a record in the access-control repository — a
// `controllers` row or a `portals` row with the identical code — and the three
// site codes go the other way, from here into that app's `locations`. Two Go
// modules cannot import each other's fixtures, so this list IS the contract:
// changing a code on either side has to change it on both, and this test is what
// says so out loud on this side. access-control's
// TestSiteCodesMatchThePlatformDemo is its mirror.
func TestTheAccessControlEstateIsPresentAndJoinable(t *testing.T) {
	app := shared

	org, err := app.FindFirstRecordByFilter("organizations", "code = 'northwind'", nil)
	if err != nil {
		t.Fatalf("northwind org: %v", err)
	}

	typeOf := map[string]string{}
	types, err := app.FindAllRecords("thing_types", dbx.NewExp("organization = {:o}", dbx.Params{"o": org.Id}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range types {
		typeOf[tt.Id] = tt.GetString("code")
	}

	// controller code -> the access-control `controllers` rows.
	// portal code     -> the access-control `portals` rows.
	want := map[string]string{
		"ctrl-kc-dc1-1":    "access-controller",
		"ctrl-kc-dc1-2":    "access-controller",
		"ctrl-kc-office-1": "access-controller",
		"ctrl-sgf-xd2-1":   "access-controller",

		"kc-dc1-main":      "access-door",
		"kc-dc1-dock-a":    "access-door",
		"kc-dc1-freezer-1": "access-door",
		"kc-dc1-mdf":       "access-door",
		"kc-dc1-yard":      "access-gate",
		"kc-office-lobby":  "access-door",
		"kc-office-server": "access-door",
		"sgf-xd2-main":     "access-door",
		"sgf-xd2-dock-b":   "access-door",
		"sgf-xd2-mdf":      "access-door",
	}

	for code, wantType := range want {
		thing, err := app.FindFirstRecordByFilter("things",
			"organization = {:o} && code = {:c}", dbx.Params{"o": org.Id, "c": code})
		if err != nil || thing == nil {
			t.Errorf("no Thing with code %q in northwind — the access-control join is broken", code)
			continue
		}
		if got := typeOf[thing.GetString("type")]; got != wantType {
			t.Errorf("Thing %q has type %q, want %q", code, got, wantType)
		}
		// A controller is a network participant and gets a Nebula host; a door is
		// I/O on that controller's terminal block and must not.
		hasNebula := thing.GetString("nebula_host") != ""
		if wantType == "access-controller" && !hasNebula {
			t.Errorf("controller %q has no Nebula host; it is the box that needs the management path", code)
		}
		if wantType != "access-controller" && hasNebula {
			t.Errorf("door %q has a Nebula host; a door is not a network participant", code)
		}
	}

	// The site codes access-control mirrors. These are the ones its own
	// TestSiteCodesMatchThePlatformDemo asserts from the other side.
	for _, code := range []string{"KC-DC1", "KC-OFFICE", "SGF-XD2"} {
		if _, err := app.FindFirstRecordByFilter("locations",
			"organization = {:o} && code = {:c}", dbx.Params{"o": org.Id, "c": code}); err != nil {
			t.Errorf("no location %q in northwind — access-control mirrors this code", code)
		}
	}
}

// A portal's subject prefix has to compose to what a controller actually
// publishes on: acc.{location}.{type}.{thing}. Getting this wrong is invisible —
// the Thing Type screen renders a plausible subject that nothing is listening to.
func TestAccessSubjectPrefixesMatchTheWireFormat(t *testing.T) {
	app := shared

	for _, tc := range []struct{ code, want string }{
		{"access-controller", "acc.{location}.ctrl.{thing}"},
		{"access-door", "acc.{location}.door.{thing}"},
		{"access-gate", "acc.{location}.gate.{thing}"},
	} {
		tt, err := app.FindFirstRecordByFilter("thing_types", "code = {:c}", dbx.Params{"c": tc.code})
		if err != nil {
			t.Fatalf("thing type %q: %v", tc.code, err)
		}
		if got := tt.GetString("subject_prefix"); got != tc.want {
			t.Errorf("%s subject_prefix = %q, want %q", tc.code, got, tc.want)
		}
	}
}

// Locations need coordinates or the map screens — two of the console's best —
// come up empty, which is the single most common reason a demo underwhelms.
func TestTopLevelLocationsHaveCoordinates(t *testing.T) {
	app := shared

	recs, err := app.FindAllRecords("locations")
	if err != nil {
		t.Fatal(err)
	}
	var placed int
	for _, l := range recs {
		if l.GetGeoPoint("coordinates").Lat != 0 {
			placed++
		}
	}
	if placed < len(recs) {
		t.Errorf("%d of %d locations have no coordinates", len(recs)-placed, len(recs))
	}
}

// Managed organizations get the export/import pair; unmanaged ones must not.
// Both states exist in the fixtures on purpose.
func TestManagedFlagIsSetOnTheOrgsThatClaimIt(t *testing.T) {
	app := shared

	want := map[string]bool{"northwind": true, "ironbridge": false, "galewind": true}
	for code, managed := range want {
		org, err := app.FindFirstRecordByFilter("organizations", "code = {:c}", dbx.Params{"c": code})
		if err != nil {
			t.Fatalf("org %q: %v", code, err)
		}
		if got := org.GetBool("managed"); got != managed {
			t.Errorf("org %q managed = %v, want %v", code, got, managed)
		}
	}
}

// Every operation that names a schema must resolve to one, and every thing type
// must resolve its operations. A dangling relation here renders as an empty
// section in the console rather than an error.
func TestTheContractGraphIsWhole(t *testing.T) {
	app := shared

	ops, err := app.FindAllRecords("thing_type_operations")
	if err != nil {
		t.Fatal(err)
	}
	withSchema := 0
	for _, op := range ops {
		if id := op.GetString("schema"); id != "" {
			if _, err := app.FindRecordById("message_schemas", id); err != nil {
				t.Errorf("operation %q points at a missing schema", op.GetString("name"))
			}
			withSchema++
		}
	}
	if withSchema == 0 {
		t.Error("no operation is linked to a message schema")
	}

	types, err := app.FindAllRecords("thing_types")
	if err != nil {
		t.Fatal(err)
	}
	linked := 0
	for _, tt := range types {
		if tt.GetString("nats_role") == "" {
			t.Errorf("thing type %q has no default NATS role", tt.GetString("code"))
		}
		if tt.GetString("subject_prefix") == "" {
			t.Errorf("thing type %q has no subject prefix", tt.GetString("code"))
		}
		if len(tt.GetStringSlice("operations")) > 0 {
			linked++
		}
	}
	if linked == 0 {
		t.Error("no thing type has any operations linked")
	}
}

// A thing type's default NATS role has to be able to carry that type's own
// contract. Nothing in this platform checks that at runtime: a role is a set of
// subject patterns on one screen, a thing type is a subject prefix plus a list
// of operations on another, and the credential minted from the pair is only
// tested by a device trying to speak and being refused by the server.
//
// It shipped wrong twice at once. The three stone-access types took the stock
// `device`/`gateway` roles, whose publish lists named every subtree in the
// fixture except `acc.>` — so a seeded door could not publish the decision its
// own type declares, and a controller could not receive the taps it exists to
// answer. Separately, `reply_diagnostics` on three types resolved to roles
// carrying `_INBOX.>` on SUBSCRIBE only, which is the requester's side of
// request/reply rather than the responder's.
//
// Both failures are invisible in the console. Every screen renders correctly and
// the JWT signs cleanly; the permission is simply absent from it.
func TestEveryThingTypeCanSpeakItsOwnContract(t *testing.T) {
	app := shared

	types, err := app.FindAllRecords("thing_types")
	if err != nil {
		t.Fatal(err)
	}
	if len(types) == 0 {
		t.Fatal("no thing types were seeded")
	}

	checked := 0
	for _, tt := range types {
		code := tt.GetString("code")

		role, err := app.FindRecordById("nats_roles", tt.GetString("nats_role"))
		if err != nil {
			t.Fatalf("thing type %q: default role: %v", code, err)
		}
		var pub, sub []string
		if err := role.UnmarshalJSONField("publish_permissions", &pub); err != nil {
			t.Fatalf("role %q publish_permissions: %v", role.GetString("name"), err)
		}
		if err := role.UnmarshalJSONField("subscribe_permissions", &sub); err != nil {
			t.Fatalf("role %q subscribe_permissions: %v", role.GetString("name"), err)
		}

		for _, opID := range tt.GetStringSlice("operations") {
			op, err := app.FindRecordById("thing_type_operations", opID)
			if err != nil {
				t.Fatalf("thing type %q: operation %s: %v", code, opID, err)
			}
			subject := composeSubject(tt.GetString("subject_prefix"), op.GetString("subject_suffix"))
			name := op.GetString("name")
			checked++

			// A responder needs the subject on subscribe and an inbox on publish;
			// a requester needs the mirror image. Getting one half is the failure
			// mode, because the half you have is the one that looks like it works.
			switch op.GetString("capability") {
			case "publish":
				requirePermission(t, code, name, "publish", subject, pub)
			case "subscribe":
				requirePermission(t, code, name, "subscribe", subject, sub)
			case "reply":
				requirePermission(t, code, name, "subscribe", subject, sub)
				requirePermission(t, code, name, "publish", "_INBOX.abc123", pub)
			case "request":
				requirePermission(t, code, name, "publish", subject, pub)
				requirePermission(t, code, name, "subscribe", "_INBOX.abc123", sub)
			default:
				t.Errorf("operation %q on %q has unknown capability %q",
					name, code, op.GetString("capability"))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no thing type had any operation to check")
	}
}

func requirePermission(t *testing.T, thingType, op, dir, subject string, patterns []string) {
	t.Helper()
	for _, p := range patterns {
		if subjectMatches(p, subject) {
			return
		}
	}
	t.Errorf("thing type %q declares operation %q but its default NATS role cannot %s %q (patterns: %v)",
		thingType, op, dir, subject, patterns)
}

// composeSubject renders a thing type's subject prefix the way the console's
// Thing Type screen does, substituting a placeholder for each template variable.
// Only the literal tokens matter for a permission check, and every prefix in the
// fixture is literal in its first token.
func composeSubject(prefix, suffix string) string {
	var b strings.Builder
	for i := 0; i < len(prefix); i++ {
		if prefix[i] != '{' {
			b.WriteByte(prefix[i])
			continue
		}
		end := strings.IndexByte(prefix[i:], '}')
		if end < 0 {
			b.WriteByte(prefix[i])
			continue
		}
		b.WriteString("X")
		i += end
	}
	if suffix == "" {
		return b.String()
	}
	return b.String() + "." + suffix
}

// subjectMatches implements NATS subject matching for a permission pattern:
// `*` matches exactly one token, `>` matches one or more trailing tokens.
func subjectMatches(pattern, subject string) bool {
	p := strings.Split(pattern, ".")
	s := strings.Split(subject, ".")
	for i, tok := range p {
		if tok == ">" {
			return i < len(s)
		}
		if i >= len(s) {
			return false
		}
		if tok != "*" && tok != s[i] {
			return false
		}
	}
	return len(p) == len(s)
}

// The three permissions an edge service needs that no thing type's operation
// list mentions, so TestEveryThingTypeCanSpeakItsOwnContract cannot derive them.
// Each was found by running the real thing — four access-controllers against a
// `serve --nats` deployment, authenticating as their own seeded Things — and each
// failed in a way that looked like someone else's bug.
//
//	$JS.API.>  bind a stream, create a consumer. Missing: no policy sync at all.
//	$KV.>      WRITE a KV value, which is a plain publish to $KV.{bucket}.{key}
//	           and is NOT covered by $JS.API.>. Missing: the controller boots
//	           clean, syncs its whole policy graph, arms every portal, and then
//	           fails on the first status write with a permissions violation. A
//	           box that starts fine and cannot report state is worse than one
//	           that refuses to start.
//	_INBOX.>   answer a request. Missing: request/reply is dead in one direction
//	           only, which is the half that looks like it works.
//
// The one to keep in mind when adding a role: reading a KV bucket and writing one
// need different permissions, and only the read goes through the JetStream API.
func TestGatewayRoleCanRunAnEdgeService(t *testing.T) {
	app := shared

	roles, err := app.FindAllRecords("nats_roles", dbx.NewExp("name = 'gateway'"))
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) == 0 {
		t.Fatal("gateway role was not seeded")
	}
	for _, r := range roles {
		var pub []string
		if err := r.UnmarshalJSONField("publish_permissions", &pub); err != nil {
			t.Fatalf("publish_permissions: %v", err)
		}
		for _, subject := range []string{
			"$JS.API.STREAM.INFO.KV_ACC_POLICY", // bind a bucket
			"$KV.ACC_STATUS.portal.kc-dc1-main", // write one
			"_INBOX.abc123",                     // answer a request
		} {
			requirePermission(t, "gateway role", "edge service", "publish", subject, pub)
		}
	}
}
