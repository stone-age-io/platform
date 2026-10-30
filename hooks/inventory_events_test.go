package hooks_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/hooks"
	"platform/internal/testutil"
)

// The columns that hold a credential somewhere in the schema. None may ever be
// on the bus: an inventory event reaches every subscriber in the tenant's
// account, which is a far wider audience than the row rules that guard these.
var credentialFields = []string{
	"password", "tokenKey", "email", "creds_file", "config_yaml", "private_key",
	"seed", "jwt", "nats_user", "nebula_host",
}

func TestInventorySubject(t *testing.T) {
	got := hooks.InventorySubject("stone-age.>", "acme", "thing", "created")
	if want := "stone-age.acme.inventory.thing.created"; got != want {
		t.Errorf("subject = %q, want %q", got, want)
	}
	// A multi-token prefix keeps every token, so a white-labelled deployment's
	// "acme.platform.>" roots at acme.platform.<code>.
	got = hooks.InventorySubject("acme.platform.>", "northwind", "location", "deleted")
	if want := "acme.platform.northwind.inventory.location.deleted"; got != want {
		t.Errorf("subject = %q, want %q", got, want)
	}
}

func TestInventoryAllowlistNamesNoCredential(t *testing.T) {
	for kind, fields := range hooks.InventoryFields {
		for _, f := range credentialFields {
			if slices.Contains(fields, f) {
				t.Errorf("the %s allowlist includes %q, a credential-bearing field", kind, f)
			}
		}
	}
}

// The console marks the inventory pair read-only by name. A drifted literal
// would offer an Edit button on records the sweep rewrites, with no error at
// either end -- the reason TestConsoleRecognisesTheManagedExportName exists for
// the helpdesk pair.
func TestConsoleRecognisesTheInventoryNames(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash(managedExportsTS))
	if err != nil {
		t.Fatalf("read %s: %v", managedExportsTS, err)
	}
	src := string(raw)
	if want := "export const INVENTORY_IMPORT_NAME = '" + hooks.InventoryImportName + "'"; !strings.Contains(src, want) {
		t.Errorf("%s does not contain %q", managedExportsTS, want)
	}
	m := regexp.MustCompile(`const INVENTORY_EXPORT_RE = /(.+)/`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("INVENTORY_EXPORT_RE not found in %s", managedExportsTS)
	}
	if name := hooks.InventoryExportName("a1b2c3d4e5f6g7h"); !regexp.MustCompile(m[1]).MatchString(name) {
		t.Errorf("the hook names the export %q, which the console's %s does not match", name, m[1])
	}
}

type inventoryEvent struct {
	subject string
	payload map[string]any
}

// inventoryRecorder stands in for the NATS connection.
type inventoryRecorder struct {
	events []inventoryEvent
	fail   bool
}

func (r *inventoryRecorder) publish(subject string, data []byte) error {
	if r.fail {
		return errors.New("nats unreachable")
	}
	var p map[string]any
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	r.events = append(r.events, inventoryEvent{subject, p})
	return nil
}

// take returns and clears what was published since the last call.
func (r *inventoryRecorder) take() []inventoryEvent {
	out := r.events
	r.events = nil
	return out
}

func changedOf(ev inventoryEvent) []string {
	raw, _ := ev.payload["changed"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.(string))
	}
	return out
}

func TestInventoryEvents(t *testing.T) {
	app := testutil.SetupApp(t)
	rec := &inventoryRecorder{}

	opts := hooks.InventoryEventsOptions{
		Enabled:            true,
		OrgCollection:      "organizations",
		AccountCollection:  "nats_accounts",
		UserCollection:     "nats_users",
		RoleCollection:     "nats_roles",
		ExportCollection:   "nats_account_exports",
		ImportCollection:   "nats_account_imports",
		ThingCollection:    "things",
		LocationCollection: "locations",
		LocalSubject:       "stone-age.>",
		Publish:            rec.publish,
	}
	// Bound after Bootstrap, beside the harness's disabled registration. A late
	// bind fires now that no organization hook terminates the chain.
	hooks.RegisterInventoryEvents(app, opts)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("collection %s: %v", collection, err)
		}
		return core.NewRecord(col)
	}
	mustSave := func(r *core.Record) *core.Record {
		t.Helper()
		if err := app.Save(r); err != nil {
			t.Fatalf("save %s: %v", r.Collection().Name, err)
		}
		return r
	}
	reload := func(collection, id string) *core.Record {
		t.Helper()
		r, err := app.FindRecordById(collection, id)
		if err != nil {
			t.Fatalf("reload %s %s: %v", collection, id, err)
		}
		return r
	}

	owner := newRecord("users")
	owner.Set("email", "inventory-owner@example.test")
	owner.Set("password", "owner-password-123")
	mustSave(owner)

	newOrg := func(name string, operator bool) *core.Record {
		r := newRecord("organizations")
		r.Set("name", name)
		r.Set("active", true)
		r.Set("owner", owner.Id)
		r.Set("is_operator_org", operator)
		return mustSave(r)
	}
	hub := newOrg("Inventory Hub", true)
	tenant := reload("organizations", newOrg("Inventory Tenant", false).Id)
	code := tenant.GetString("code")
	if code == "" {
		t.Fatal("tenant has no code; RegisterOrgCode should have generated one")
	}

	accountOf := func(org *core.Record) *core.Record {
		t.Helper()
		r, err := app.FindFirstRecordByFilter("nats_accounts", "organization = {:o}", dbx.Params{"o": org.Id})
		if err != nil {
			t.Fatalf("NATS account for %s: %v", org.GetString("name"), err)
		}
		return r
	}
	findPair := func() (exp, imp *core.Record) {
		exp, _ = app.FindFirstRecordByFilter("nats_account_exports", "name = {:n}",
			dbx.Params{"n": hooks.InventoryExportName(tenant.Id)})
		imp, _ = app.FindFirstRecordByFilter("nats_account_imports", "account_id = {:a} && name = {:n}",
			dbx.Params{"a": accountOf(tenant).Id, "n": hooks.InventoryImportName})
		return exp, imp
	}

	t.Run("an org create wires the pair across the two accounts", func(t *testing.T) {
		exp, imp := findPair()
		if exp == nil || imp == nil {
			t.Fatalf("export present = %v, import present = %v; want both", exp != nil, imp != nil)
		}
		wantSubject := "stone-age." + code + ".>"
		if exp.GetString("account_id") != accountOf(hub).Id {
			t.Error("the export is not on the provider account")
		}
		if exp.GetString("organization") != hub.Id {
			t.Error("the export does not carry the operator org, which owns its account")
		}
		if exp.GetString("subject") != wantSubject {
			t.Errorf("export subject = %q, want %q", exp.GetString("subject"), wantSubject)
		}
		if imp.GetString("organization") != tenant.Id {
			t.Error("the import does not carry the tenant org, so the tenant cannot list it")
		}
		if imp.GetString("subject") != wantSubject || imp.GetString("local_subject") != "stone-age.>" {
			t.Errorf("import maps %q to %q, want %q to stone-age.>",
				imp.GetString("subject"), imp.GetString("local_subject"), wantSubject)
		}
		if imp.GetString("account") != accountOf(hub).GetString("public_key") {
			t.Error("the import does not name the provider account's public key")
		}
	})

	t.Run("re-saving and sweeping write nothing when the pair is right", func(t *testing.T) {
		exp, imp := findPair()
		mustSave(reload("organizations", tenant.Id))
		hooks.SweepInventoryExports(app, opts)
		exp2, imp2 := findPair()
		if exp2.GetString("updated") != exp.GetString("updated") || imp2.GetString("updated") != imp.GetString("updated") {
			t.Error("a no-op reconcile saved a record, which republishes the account JWT")
		}
	})

	t.Run("a drifted import is reconciled back", func(t *testing.T) {
		_, imp := findPair()
		imp.Set("local_subject", "elsewhere.>")
		mustSave(imp)
		hooks.SweepInventoryExports(app, opts)
		if _, imp = findPair(); imp.GetString("local_subject") != "stone-age.>" {
			t.Errorf("local_subject = %q after the sweep, want stone-age.>", imp.GetString("local_subject"))
		}
	})

	t.Run("the infrastructure orgs get no pair", func(t *testing.T) {
		if r, _ := app.FindFirstRecordByFilter("nats_account_exports", "name = {:n}",
			dbx.Params{"n": hooks.InventoryExportName(hub.Id)}); r != nil {
			t.Error("the operator org exported inventory to itself")
		}
	})

	t.Run("the platform-events identity is created once", func(t *testing.T) {
		creds, err := hooks.EnsurePlatformEventsUser(app, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(creds, "NATS USER JWT") {
			t.Error("creds_file does not look like a .creds file")
		}
		if _, err := hooks.EnsurePlatformEventsUser(app, opts); err != nil {
			t.Fatal(err)
		}
		users, _ := app.FindAllRecords("nats_users", dbx.HashExp{"nats_username": "platform-events"})
		if len(users) != 1 {
			t.Fatalf("%d platform-events users, want 1", len(users))
		}
		if users[0].GetString("account_id") != accountOf(hub).Id {
			t.Error("platform-events is not in the provider account")
		}
	})

	rec.take()

	newLocation := func(c string, parent *core.Record) *core.Record {
		r := newRecord("locations")
		r.Set("name", c)
		r.Set("code", c)
		r.Set("organization", tenant.Id)
		if parent != nil {
			r.Set("parent", parent.Id)
		}
		return mustSave(r)
	}
	hq := newLocation("HQ", nil)
	floor := newLocation("FL1", hq)

	thing := newRecord("things")
	thing.Set("organization", tenant.Id)
	thing.Set("name", "Lobby Camera")
	thing.Set("code", "CAM-LOBBY")
	thing.Set("email", "cam-lobby@things.inventory.test")
	thing.Set("emailVisibility", true)
	thing.Set("password", "probe-password-123")
	thing.Set("location", floor.Id)
	mustSave(thing)

	t.Run("creates publish the whole record on the tenant's subject", func(t *testing.T) {
		events := rec.take()
		if len(events) != 3 {
			t.Fatalf("%d events, want 3 (two locations, one thing): %v", len(events), events)
		}
		ev := events[2]
		if want := "stone-age." + code + ".inventory.thing.created"; ev.subject != want {
			t.Errorf("subject = %q, want %q", ev.subject, want)
		}
		if ev.payload["code"] != "CAM-LOBBY" || ev.payload["location"] != "FL1" {
			t.Errorf("payload code/location = %v/%v, want CAM-LOBBY/FL1", ev.payload["code"], ev.payload["location"])
		}
		if ev.payload["location_path"] != "/HQ/FL1/" {
			t.Errorf("location_path = %v, want /HQ/FL1/", ev.payload["location_path"])
		}
		if _, ok := ev.payload["changed"]; ok {
			t.Error("a created event carries `changed`")
		}
		for _, f := range credentialFields {
			if _, ok := ev.payload[f]; ok {
				t.Errorf("payload carries credential-bearing field %q", f)
			}
		}
	})

	t.Run("an update lists exactly what moved", func(t *testing.T) {
		r := reload("things", thing.Id)
		r.Set("name", "Lobby Camera 2")
		mustSave(r)
		events := rec.take()
		if len(events) != 1 || !slices.Equal(changedOf(events[0]), []string{"name"}) {
			t.Fatalf("events = %v, want one with changed [name]", events)
		}
		if events[0].payload["location_path"] != "/HQ/FL1/" {
			t.Error("an update is not the whole record: location_path is missing")
		}
	})

	t.Run("a write outside the allowlist publishes nothing", func(t *testing.T) {
		r := reload("things", thing.Id)
		r.Set("floorplan_position", map[string]any{"x": 1, "y": 2})
		mustSave(r)
		if events := rec.take(); len(events) != 0 {
			t.Errorf("events = %v, want none", events)
		}
	})

	t.Run("deactivation is an update whose changed names active", func(t *testing.T) {
		r := reload("things", thing.Id)
		r.Set("active", false)
		mustSave(r)
		events := rec.take()
		if len(events) != 1 || !slices.Contains(changedOf(events[0]), "active") {
			t.Fatalf("events = %v, want one naming active", events)
		}
		if !strings.HasSuffix(events[0].subject, ".inventory.thing.updated") {
			t.Errorf("subject = %q, want an updated event", events[0].subject)
		}
	})

	t.Run("one record saved twice over-reports on the second event", func(t *testing.T) {
		// Record.Original() is the last READ (hooks/CLAUDE.md), so the second
		// save compares against the state before the first. Known, and safe.
		r := reload("things", thing.Id)
		r.Set("name", "Lobby Camera 3")
		mustSave(r)
		r.Set("description", "east wall")
		mustSave(r)
		events := rec.take()
		if len(events) != 2 {
			t.Fatalf("%d events, want 2", len(events))
		}
		second := changedOf(events[1])
		if !slices.Contains(second, "description") || !slices.Contains(second, "name") {
			t.Errorf("second changed = %v, want both name and description", second)
		}
	})

	t.Run("a move republishes the subtree with its new paths", func(t *testing.T) {
		wh := newLocation("WH", nil)
		rec.take()

		r := reload("locations", hq.Id)
		r.Set("parent", wh.Id)
		mustSave(r)

		byCode := map[string]inventoryEvent{}
		for _, ev := range rec.take() {
			byCode[ev.payload["code"].(string)] = ev
		}
		if ev, ok := byCode["HQ"]; !ok || !slices.Contains(changedOf(ev), "path") {
			t.Errorf("moved location event = %v, want one naming path", ev)
		}
		if ev, ok := byCode["FL1"]; !ok || ev.payload["path"] != "/WH/HQ/FL1/" {
			t.Errorf("descendant event = %v, want path /WH/HQ/FL1/", ev)
		}
		ev, ok := byCode["CAM-LOBBY"]
		if !ok || ev.payload["location_path"] != "/WH/HQ/FL1/" ||
			!slices.Equal(changedOf(ev), []string{"location_path"}) {
			t.Errorf("thing event = %v, want location_path /WH/HQ/FL1/ with changed [location_path]", ev)
		}
	})

	t.Run("deleting a location updates the Things that pointed at it", func(t *testing.T) {
		if err := app.Delete(reload("locations", floor.Id)); err != nil {
			t.Fatal(err)
		}
		var thingEvent, deleted *inventoryEvent
		for _, ev := range rec.take() {
			switch {
			case ev.payload["code"] == "CAM-LOBBY":
				thingEvent = &ev
			case strings.HasSuffix(ev.subject, ".inventory.location.deleted"):
				deleted = &ev
			}
		}
		if deleted == nil || deleted.payload["code"] != "FL1" {
			t.Errorf("no location.deleted event for FL1")
		}
		if thingEvent == nil {
			t.Fatal("no event for the Thing whose location was deleted")
		}
		if got := changedOf(*thingEvent); !slices.Contains(got, "location") || !slices.Contains(got, "location_path") {
			t.Errorf("thing changed = %v, want location and location_path", got)
		}
		if thingEvent.payload["location"] != "" || thingEvent.payload["location_path"] != "" {
			t.Errorf("thing still points at a location: %v", thingEvent.payload)
		}
	})

	t.Run("a publish failure does not fail the write", func(t *testing.T) {
		rec.fail = true
		defer func() { rec.fail = false }()
		r := reload("things", thing.Id)
		r.Set("name", "Saved Anyway")
		if err := app.Save(r); err != nil {
			t.Fatalf("save failed because the publish did: %v", err)
		}
		if reload("things", thing.Id).GetString("name") != "Saved Anyway" {
			t.Error("the write did not persist")
		}
	})

	t.Run("turning the flag off removes the pair; deleting the org removes the export", func(t *testing.T) {
		off := opts
		off.Enabled = false
		hooks.SweepInventoryExports(app, off)
		if exp, imp := findPair(); exp != nil || imp != nil {
			t.Errorf("after a disabled sweep: export present = %v, import present = %v", exp != nil, imp != nil)
		}

		hooks.SweepInventoryExports(app, opts)
		if exp, _ := findPair(); exp == nil {
			t.Fatal("re-enabling did not rewire the pair")
		}

		// An org with no inventory: deleting one that has nested locations
		// blanks their organization and trips relation tenancy on the parent
		// link, which is the existing reason org delete is operator-only and
		// not what is under test here.
		doomed := newOrg("Inventory Doomed", false)
		exportOf := func() *core.Record {
			r, _ := app.FindFirstRecordByFilter("nats_account_exports", "name = {:n}",
				dbx.Params{"n": hooks.InventoryExportName(doomed.Id)})
			return r
		}
		if exportOf() == nil {
			t.Fatal("the new org got no export")
		}
		if err := app.Delete(reload("organizations", doomed.Id)); err != nil {
			t.Fatal(err)
		}
		if exportOf() != nil {
			t.Error("the provider-side export outlived its organization")
		}
	})
}
