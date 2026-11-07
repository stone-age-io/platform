package demoseed_test

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/demoseed"
	"platform/internal/testutil"
)

// A modest fleet keeps the suite quick; every property asserted below is
// independent of scale.
const testThings = 60

// One seeded app, shared by every read-only test in this file, because booting
// and seeding costs several seconds.
//
// The tests that mutate build their own app with testutil.SetupApp, because
// sharing one would make them order-dependent.
var shared core.App

func TestMain(m *testing.M) {
	// flag.Parse() first: m.Run() normally parses flags, so without it
	// testing.Short() is always false here.
	flag.Parse()
	if testing.Short() {
		// Seed nothing: sharedApp(t) and testutil.SetupApp both skip.
		os.Exit(m.Run())
	}

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

// sharedApp is how every read-only test reaches the seeded app. Under -short
// the app is never built, and reading `shared` directly would nil-deref rather
// than skip.
func sharedApp(t *testing.T) core.App {
	t.Helper()
	testutil.SkipIfShort(t)
	if shared == nil {
		t.Fatal("the shared seeded app was not built; TestMain should have failed loudly")
	}
	return shared
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
	app := sharedApp(t)

	for _, c := range []struct {
		name string
		min  int
	}{
		{"organizations", 3},
		{"users", 11},
		{"memberships", 13},
		{"location_types", 13},
		{"locations", 25},
		{"thing_type_operations", 35},
		{"thing_types", 22},
		{"things", testThings},
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

// The seed is built on organizations.code, which is immutable once written, so
// a wrong one could not be repaired in place.
func TestOrganizationsCarryTheCodesTheHelpdeskDemoJoinsOn(t *testing.T) {
	app := sharedApp(t)

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
// account and Nebula CA. A failure means the hook is not registered, and the
// demo has no credentials behind it.
func TestEveryOrganizationIsProvisioned(t *testing.T) {
	app := sharedApp(t)

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

// things.authRule is `active = true`, and a PocketBase bool has no schema
// default, so a Thing seeded without the flag would be locked out of the API.
func TestEveryActiveThingCanActuallyAuthenticate(t *testing.T) {
	app := sharedApp(t)

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
	// Decommissioned devices are seeded on purpose.
	if inactive == 0 {
		t.Error("no inactive things — the decommissioned state is unrepresented")
	}
}

// Deactivating a Thing has to reach nats_users and nebula_hosts, or it has only
// closed the console door.
//
// This asserts the effect, not the flag: JWT generation ignores
// nats_users.active, and pb-nats's suspend adds the public key to the owning
// account's revocation list. The seeder never writes that list, so the check
// cannot pass by accident.
func TestDecommissionedThingsHaveTheirCredentialRevoked(t *testing.T) {
	app := sharedApp(t)

	inactive, err := app.FindAllRecords("things", dbx.NewExp("active = false"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inactive) == 0 {
		// Not t.Skip: a test that skips when its subject is missing cannot
		// report that it is missing.
		t.Fatal("no decommissioned things, so this assertion never ran: the seeder produced none")
	}

	for _, thing := range inactive {
		code := thing.GetString("code")

		u, err := app.FindRecordById("nats_users", thing.GetString("nats_user"))
		if err != nil {
			t.Errorf("thing %q: %v", code, err)
			continue
		}
		if u.GetBool("active") {
			t.Errorf("thing %q is decommissioned but its NATS identity is still active", code)
		}

		pubKey := u.GetString("public_key")
		if pubKey == "" {
			t.Errorf("thing %q: its NATS identity has no public key to revoke", code)
			continue
		}
		acct, err := app.FindRecordById("nats_accounts", u.GetString("account_id"))
		if err != nil {
			t.Errorf("thing %q: cannot load the owning NATS account: %v", code, err)
			continue
		}
		if !strings.Contains(acct.GetString("revocations"), pubKey) {
			t.Errorf("thing %q is decommissioned but its public key is absent from account %q's "+
				"revocation list -- the credential still works", code, acct.GetString("name"))
		}

		// A Thing with a Nebula host must have it blocklisted, or it keeps
		// overlay access until its certificate expires.
		if hostID := thing.GetString("nebula_host"); hostID != "" {
			h, err := app.FindRecordById("nebula_hosts", hostID)
			if err != nil {
				t.Errorf("thing %q: cannot load its Nebula host: %v", code, err)
			} else if h.GetBool("active") {
				t.Errorf("thing %q is decommissioned but its Nebula host is still active", code)
			}
		}
	}
}

// Every one of the five console roles should be reachable by logging in, or the
// demo cannot show what they differ on.
func TestAllFiveConsoleRolesAreRepresented(t *testing.T) {
	app := sharedApp(t)

	seen := map[string]bool{}
	memberships, err := app.FindAllRecords("memberships")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range memberships {
		seen[m.GetString("role")] = true
		// Every membership is paired with a NATS identity.
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

// The read-only console roles must be paired with a read-only NATS role, or a
// viewer could publish anywhere on the bus.
func TestReadOnlyConsoleRolesGetAReadOnlyNatsRole(t *testing.T) {
	app := sharedApp(t)

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

// The read-only NATS role must not carry a publish wildcard.
func TestConsoleReadonlyRoleCannotPublishAnywhere(t *testing.T) {
	app := sharedApp(t)

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

// Re-running must converge, not duplicate.
func TestSeedingTwiceChangesNothing(t *testing.T) {
	app := testutil.SetupApp(t)

	first, err := demoseed.Run(app, demoseed.Options{Things: testThings})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	before := map[string]int{}
	for _, c := range []string{"organizations", "things", "locations", "nats_users", "memberships"} {
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
	app := sharedApp(t)

	for _, collection := range []string{"things", "locations", "thing_types", "location_types"} {
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

// Things span more than sensors: an application is a first-class participant
// with its own signed identity.
func TestThingsSpanDevicesGatewaysAndApplications(t *testing.T) {
	app := sharedApp(t)

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
		case "wms-connector", "coldchain-rules", "kiosk-controller", "oee-analytics", "mes-connector", "scada-bridge", "market-feed":
			kinds["application"] = true
		case "edge-gateway", "access-controller", "tool-kiosk":
			kinds["gateway"] = true
		case "dock-display", "timeclock-terminal", "ops-wallboard":
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
// Every code below is also a `controllers` or `portals` row in the
// access-control repo, and the three site codes go the other way, into its
// `locations`. The modules share no fixtures, so this list is the contract:
// change a code on both sides. access-control's
// TestSiteCodesMatchThePlatformDemo is its mirror.
func TestTheAccessControlEstateIsPresentAndJoinable(t *testing.T) {
	app := sharedApp(t)

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
		// A controller gets a Nebula host; a door, I/O on its terminal block, must not.
		hasNebula := thing.GetString("nebula_host") != ""
		if wantType == "access-controller" && !hasNebula {
			t.Errorf("controller %q has no Nebula host; it is the box that needs the management path", code)
		}
		if wantType != "access-controller" && hasNebula {
			t.Errorf("door %q has a Nebula host; a door is not a network participant", code)
		}
	}

	// The site codes access-control mirrors (its TestSiteCodesMatchThePlatformDemo).
	for _, code := range []string{"KC-DC1", "KC-OFFICE", "SGF-XD2"} {
		if _, err := app.FindFirstRecordByFilter("locations",
			"organization = {:o} && code = {:c}", dbx.Params{"o": org.Id, "c": code}); err != nil {
			t.Errorf("no location %q in northwind — access-control mirrors this code", code)
		}
	}
}

// A portal's subject prefix has to compose to what a controller publishes on:
// acc.{location}.{type}.{thing}. A wrong one still renders a plausible subject.
func TestAccessSubjectPrefixesMatchTheWireFormat(t *testing.T) {
	app := sharedApp(t)

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

// The kiosk join. Every code here is also a `kiosks` row in the kiosk repo's
// internal/demoseed/data.go; this list is the contract on this side.
//
// Ports are asserted because the demo estate runs on one host, and two nodes on
// the same port means one dies at startup with address-in-use.
func TestTheKioskEstateIsPresentAndJoinable(t *testing.T) {
	app := sharedApp(t)

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

	want := []struct {
		code, thingType string
		port            float64 // 0 = this thing binds no port
	}{
		{"KC-DC1-CRIB", "tool-kiosk", 8101},
		{"KC-DC1-DOCK", "tool-kiosk", 8102},
		{"SGF-XD2-CRIB", "tool-kiosk", 8103},
		{"KC-OFFICE-TC", "timeclock-terminal", 8092},
		{"KIOSK-CTRL-01", "kiosk-controller", 0},
	}

	seenPort := map[float64]string{}
	for _, w := range want {
		thing, err := app.FindFirstRecordByFilter("things",
			"organization = {:o} && code = {:c}", dbx.Params{"o": org.Id, "c": w.code})
		if err != nil || thing == nil {
			t.Errorf("no Thing with code %q in northwind — the kiosk join is broken", w.code)
			continue
		}
		if got := typeOf[thing.GetString("type")]; got != w.thingType {
			t.Errorf("Thing %q has type %q, want %q", w.code, got, w.thingType)
		}
		// Unlike a door, every participant here is a computer an admin must reach.
		if thing.GetString("nebula_host") == "" {
			t.Errorf("%q has no Nebula host; every kiosk node is a mini-PC with its own database", w.code)
		}

		var meta map[string]any
		if err := thing.UnmarshalJSONField("metadata", &meta); err != nil {
			t.Errorf("%q metadata: %v", w.code, err)
			continue
		}
		if w.port == 0 {
			continue
		}
		got, _ := meta["port"].(float64)
		if got != w.port {
			t.Errorf("%q binds port %v, want %v", w.code, got, w.port)
		}
		if other, dup := seenPort[got]; dup {
			t.Errorf("%q and %q both bind port %v; they share a host", w.code, other, got)
		}
		seenPort[got] = w.code
	}
}

// A kiosk's subject prefix has to compose to what a kiosk publishes on:
// kiosk.{kiosk_code}.{family}.{...}, with no location segment. The code is the
// routing token, and the controller's stream binds `kiosk.*.event.>`.
func TestKioskSubjectPrefixesMatchTheWireFormat(t *testing.T) {
	app := sharedApp(t)

	for _, code := range []string{"tool-kiosk", "timeclock-terminal"} {
		tt, err := app.FindFirstRecordByFilter("thing_types", "code = {:c}", dbx.Params{"c": code})
		if err != nil {
			t.Fatalf("thing type %q: %v", code, err)
		}
		if got := tt.GetString("subject_prefix"); got != "kiosk.{thing}" {
			t.Errorf("%s subject_prefix = %q, want %q", code, got, "kiosk.{thing}")
		}
	}
}

// Locations need coordinates or the map screens come up empty.
func TestTopLevelLocationsHaveCoordinates(t *testing.T) {
	app := sharedApp(t)

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
	app := sharedApp(t)

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

// Every operation needs a subject suffix, every thing type a subject prefix,
// and at least one type must link operations. A dangling relation renders as an empty console
// section, not an error.
func TestTheContractGraphIsWhole(t *testing.T) {
	app := sharedApp(t)

	ops, err := app.FindAllRecords("thing_type_operations")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("no operations were seeded")
	}
	// An empty subject suffix silently addresses the Thing Type's prefix itself.
	for _, op := range ops {
		if op.GetString("subject_suffix") == "" {
			t.Errorf("operation %q has no subject_suffix", op.GetString("name"))
		}
	}

	types, err := app.FindAllRecords("thing_types")
	if err != nil {
		t.Fatal(err)
	}
	linked := 0
	for _, tt := range types {
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

// A thing type's intended NATS role has to carry that type's own contract.
// Nothing checks this at runtime: role and thing type are edited on different
// screens, the JWT signs cleanly, and only the server refuses the device.
//
// It reads fixtures, not records: thing_types has no nats_role column, so the
// pairing comes from ThingTypeFixture.Role and the permissions from
// RoleTemplates, which seedNatsRoles writes verbatim into every org.
func TestEveryThingTypeCanSpeakItsOwnContract(t *testing.T) {
	if len(demoseed.ThingTypeFixtures) == 0 {
		t.Fatal("no thing type fixtures")
	}

	checked := 0
	for _, ttf := range demoseed.ThingTypeFixtures {
		if ttf.Role == "" {
			t.Errorf("thing type %q names no default NATS role", ttf.Code)
			continue
		}
		role := roleTemplateByName(ttf.Role)
		if role == nil {
			t.Errorf("thing type %q names role %q, which is not in roleTemplates", ttf.Code, ttf.Role)
			continue
		}

		for _, opName := range ttf.Operations {
			op := operationFixtureByName(ttf.Org, opName)
			if op == nil {
				t.Errorf("thing type %q links operation %q, which is not in operations", ttf.Code, opName)
				continue
			}
			subject := composeSubject(ttf.SubjectPrefix, op.SubjectSuffix)
			checked++

			// A responder needs the subject on subscribe and an inbox on publish;
			// a requester needs the mirror image.
			switch op.Capability {
			case "publish":
				requirePermission(t, ttf.Code, opName, "publish", subject, role.Publish)
			case "subscribe":
				requirePermission(t, ttf.Code, opName, "subscribe", subject, role.Subscribe)
			case "reply":
				requirePermission(t, ttf.Code, opName, "subscribe", subject, role.Subscribe)
				requirePermission(t, ttf.Code, opName, "publish", "_INBOX.abc123", role.Publish)
			case "request":
				requirePermission(t, ttf.Code, opName, "publish", subject, role.Publish)
				requirePermission(t, ttf.Code, opName, "subscribe", "_INBOX.abc123", role.Subscribe)
			default:
				t.Errorf("operation %q on %q has unknown capability %q", opName, ttf.Code, op.Capability)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no thing type had any operation to check")
	}
}

// roleTemplateByName finds a role template by name. RoleTemplates is one set
// written into every org by seedNatsRoles, so the name alone identifies it.
func roleTemplateByName(name string) *demoseed.RoleFixture {
	for i := range demoseed.RoleTemplates {
		if demoseed.RoleTemplates[i].Name == name {
			return &demoseed.RoleTemplates[i]
		}
	}
	return nil
}

// operationFixtureByName finds an operation within one org. Names are unique per
// (organization, name), matching the collection's unique index.
func operationFixtureByName(org, name string) *demoseed.OperationFixture {
	for i := range demoseed.OperationFixtures {
		if demoseed.OperationFixtures[i].Org == org && demoseed.OperationFixtures[i].Name == name {
			return &demoseed.OperationFixtures[i]
		}
	}
	return nil
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

// The three permissions an edge service needs that no operation list mentions,
// so TestEveryThingTypeCanSpeakItsOwnContract cannot derive them:
//
//	$JS.API.>  bind a stream, create a consumer.
//	$KV.>      write a KV value: a plain publish to $KV.{bucket}.{key}, not
//	           covered by $JS.API.>. Only a KV read goes through the API.
//	_INBOX.>   answer a request.
func TestGatewayRoleCanRunAnEdgeService(t *testing.T) {
	app := sharedApp(t)

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

// The kiosk controller's three permissions. Its type declares no operations
// (every subject it touches belongs to a kiosk), so
// TestEveryThingTypeCanSpeakItsOwnContract derives nothing for it and this is
// the only check.
//
//	kiosk.>     request a command at a node. Missing, it reads as "kiosk offline".
//	$JS.API.>   bind the event stream and the KV buckets.
//	$KV.>       write a KV value (the catalogue fan-out).
//
// Subscribe is `>` on this role, so only publish can be short.
func TestApplicationRoleCanRunTheKioskController(t *testing.T) {
	app := sharedApp(t)

	roles, err := app.FindAllRecords("nats_roles", dbx.NewExp("name = 'application'"))
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) == 0 {
		t.Fatal("application role was not seeded")
	}
	for _, r := range roles {
		var pub []string
		if err := r.UnmarshalJSONField("publish_permissions", &pub); err != nil {
			t.Fatalf("publish_permissions: %v", err)
		}
		for _, subject := range []string{
			"kiosk.KC-DC1-CRIB.command.inventory.adjust", // drive a node
			"$JS.API.STREAM.INFO.KIOSK_EVENTS",           // bind the ledger stream
			"$KV.catalog_items.KC-DC1-CRIB.HT-1010",      // fan the catalogue out
		} {
			requirePermission(t, "application role", "kiosk controller", "publish", subject, pub)
		}
	}
}

// The kiosk's own side of that pair: a node publishes on its subtree and
// subscribes its commands and sightings, all under `kiosk.>`. Without it the
// ledger commits locally, the publish fails, and the controller stays empty.
func TestGatewayRoleCanRunAKioskNode(t *testing.T) {
	app := sharedApp(t)

	roles, err := app.FindAllRecords("nats_roles", dbx.NewExp("name = 'gateway'"))
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) == 0 {
		t.Fatal("gateway role was not seeded")
	}
	for _, r := range roles {
		var pub, sub []string
		if err := r.UnmarshalJSONField("publish_permissions", &pub); err != nil {
			t.Fatalf("publish_permissions: %v", err)
		}
		if err := r.UnmarshalJSONField("subscribe_permissions", &sub); err != nil {
			t.Fatalf("subscribe_permissions: %v", err)
		}
		requirePermission(t, "gateway role", "kiosk node", "publish",
			"kiosk.KC-DC1-CRIB.event.transaction.complete", pub)
		requirePermission(t, "gateway role", "kiosk node", "publish",
			"kiosk.KC-DC1-CRIB.heartbeat", pub)
		requirePermission(t, "gateway role", "kiosk node", "subscribe",
			"kiosk.KC-DC1-CRIB.command.inventory.adjust", sub)
		requirePermission(t, "gateway role", "kiosk node", "subscribe",
			"kiosk.KC-DC1-CRIB.sighting.raw", sub)
	}
}

// A re-seed has to repair a role whose permissions have fallen behind the
// fixture (see reconcileRolePermissions), so a fixture change reaches installs
// that already ran the seed. The bug lives only on the second run, so the test
// seeds, damages, and seeds again.
func TestReseedRepairsARoleThatFellBehind(t *testing.T) {
	app := testutil.SetupApp(t)
	if _, err := demoseed.Run(app, demoseed.Options{Things: 10}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// The list deployments seeded before the kiosk estate.
	stale := []string{"app.>", "cmd.>", "helpdesk.>", "$JS.API.>"}
	rec, err := app.FindFirstRecordByFilter("nats_roles",
		"name = 'application' && organization.code = 'northwind'", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec.Set("publish_permissions", stale)
	if err := app.Save(rec); err != nil {
		t.Fatal(err)
	}

	if _, err := demoseed.Run(app, demoseed.Options{Things: 10}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	after, err := app.FindFirstRecordByFilter("nats_roles",
		"name = 'application' && organization.code = 'northwind'", nil)
	if err != nil {
		t.Fatal(err)
	}
	var pub []string
	if err := after.UnmarshalJSONField("publish_permissions", &pub); err != nil {
		t.Fatal(err)
	}
	// The three the kiosk controller cannot work without, per
	// TestApplicationRoleCanRunTheKioskController.
	for _, subject := range []string{
		"kiosk.KC-DC1-CRIB.command.inventory.adjust",
		"$KV.catalog_items.KC-DC1-CRIB.HT-1010",
		"$JS.API.STREAM.INFO.KIOSK_EVENTS",
	} {
		requirePermission(t, "application role", "after re-seed", "publish", subject, pub)
	}
}

// A re-seed with nothing to repair must not write: pb-nats re-signs the JWT of
// every user holding a role on each role update.
func TestReseedLeavesAnUpToDateRoleAlone(t *testing.T) {
	app := testutil.SetupApp(t)
	if _, err := demoseed.Run(app, demoseed.Options{Things: 10}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	rec, err := app.FindFirstRecordByFilter("nats_roles",
		"name = 'application' && organization.code = 'northwind'", nil)
	if err != nil {
		t.Fatal(err)
	}
	before := rec.GetDateTime("updated").String()

	if _, err := demoseed.Run(app, demoseed.Options{Things: 10}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	after, err := app.FindFirstRecordByFilter("nats_roles",
		"name = 'application' && organization.code = 'northwind'", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.GetDateTime("updated").String(); got != before {
		t.Errorf("role was rewritten by a no-op re-seed: updated %s -> %s", before, got)
	}
}
