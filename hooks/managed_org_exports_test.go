package hooks_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// WHY THIS TEST EXISTS
//
// The console presents the managed export/import pair as read-only, and it
// recognises them BY NAME -- the same identity ensureManagedExports looks them
// up by. managedExportName is an unexported Go constant, so the TypeScript side
// is a hand-copied literal in ui/src/utils/managedExports.ts.
//
// A hand-copied literal that drifts fails in the worst available way: the
// console offers an Edit button, the operator changes the subject, pb-nats
// re-signs the account JWT, and the next save of that organization puts it
// back. No error at either end. The same shape as the bugs this repo already
// writes down -- a flag PocketBase discards, a trigger field bound to the wrong
// hook, a re-import that silently keeps the live field.
//
// So this asserts the names the hook ACTUALLY creates against the literals the
// console ACTUALLY ships, by reading the .ts file. Same trick as stone-cli's
// vendored schema: the copy is fine, the copy going stale is not.
//
// NOTE ON THE HOOK ITSELF: RegisterManagedOrgExports is bound by
// internal/testutil, not here, and it has to be -- pb-tenancy terminates the
// organizations AfterCreateSuccess chain, so a late bind never fires at all.
// The comment in testutil.go has the detail.

const managedExportsTS = "../ui/src/utils/managedExports.ts"

// Mirrors the export in managedExports.ts. Parsed rather than restated, so a
// changed constant fails here instead of being duplicated in a third place.
var tsConstRe = regexp.MustCompile(`export const MANAGED_EXPORT_NAME = '([^']+)'`)

func consoleExportName(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(managedExportsTS))
	if err != nil {
		t.Fatalf("read %s: %v", managedExportsTS, err)
	}
	m := tsConstRe.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("MANAGED_EXPORT_NAME not found in %s.\n"+
			"If it moved or was reformatted, update tsConstRe -- do NOT delete this test: it is the "+
			"only thing tying the console's read-only marker to the hook that creates these records.",
			managedExportsTS)
	}
	return string(m[1])
}

// One app for the whole file: standing one up runs every migration and
// bootstraps four libraries, and these are all read-only assertions over the
// same provisioned pair.
type managedFixture struct {
	app      core.App
	tenant   *core.Record
	hub      *core.Record
	exported *core.Record
	imported *core.Record

	// Why the provisioned pair could not be loaded, if it could not. Surfaced
	// by requirePair as a FAILURE and never a skip: a skipped test is a test
	// that cannot fail, and the first version of this file passed cleanly with
	// a deliberately drifted constant because -run had filtered out the one
	// test that populated the fixture.
	setupErr string
}

var managed managedFixture

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "managed-exports")
	if err != nil {
		panic(err)
	}
	app, err := testutil.NewApp(dir)
	if err != nil {
		panic(err)
	}

	code := func() int {
		defer os.RemoveAll(dir)
		defer app.ResetBootstrapState()

		mustSave := func(rec *core.Record) *core.Record {
			if err := app.Save(rec); err != nil {
				panic(err)
			}
			return rec
		}
		newRecord := func(collection string) *core.Record {
			col, err := app.FindCollectionByNameOrId(collection)
			if err != nil {
				panic(err)
			}
			return core.NewRecord(col)
		}

		// organizations.owner is required, so an org cannot exist without one.
		owner := newRecord("users")
		owner.Set("email", "owner@example.test")
		owner.Set("password", "owner-password-123")
		mustSave(owner)

		// The hub. ensureManagedExports refuses to run without an operator org
		// that already has a NATS account, so it has to exist first.
		hub := newRecord("organizations")
		hub.Set("name", "Operator Co")
		hub.Set("active", true)
		hub.Set("owner", owner.Id)
		hub.Set("is_operator_org", true)
		mustSave(hub)

		tenant := newRecord("organizations")
		tenant.Set("name", "Acme Industries")
		tenant.Set("active", true)
		tenant.Set("owner", owner.Id)
		tenant.Set("managed", true)
		mustSave(tenant)

		managed = managedFixture{app: app, tenant: tenant, hub: hub}

		// The hook logs and swallows its own failures -- a provisioning problem
		// must not undo the organization create -- so an absent record is the
		// only signal there is. Recorded rather than panicked so the failure
		// reads as a test failure with a message.
		if exports, err := app.FindAllRecords("nats_account_exports"); err != nil {
			managed.setupErr = "list exports: " + err.Error()
		} else if len(exports) != 1 {
			managed.setupErr = fmt.Sprintf("want exactly 1 export provisioned, got %d -- the hook logged and swallowed a failure", len(exports))
		} else {
			managed.exported = exports[0]
		}
		if imports, err := app.FindAllRecords("nats_account_imports"); err != nil {
			managed.setupErr = "list imports: " + err.Error()
		} else if len(imports) != 1 {
			managed.setupErr = fmt.Sprintf("want exactly 1 import provisioned, got %d", len(imports))
		} else {
			managed.imported = imports[0]
		}

		return m.Run()
	}()

	os.Exit(code)
}

// requirePair fails -- never skips -- when TestMain could not load the pair.
func requirePair(t *testing.T) managedFixture {
	t.Helper()
	if managed.setupErr != "" {
		t.Fatalf("fixture setup: %s", managed.setupErr)
	}
	if managed.exported == nil || managed.imported == nil {
		t.Fatal("fixture setup: the managed export/import pair was not provisioned")
	}
	return managed
}

func TestConsoleRecognisesTheManagedExportName(t *testing.T) {
	f := requirePair(t)

	want := consoleExportName(t)
	if got := f.exported.GetString("name"); got != want {
		t.Errorf("the hook names the export %q but the console looks for %q.\n"+
			"The console would offer an Edit button on a record the reconciler overwrites, with no error at either end.\n"+
			"Fix MANAGED_EXPORT_NAME in %s, or managedExportName in hooks/managed_org_exports.go.",
			got, want, managedExportsTS)
	}
}

func TestConsoleRecognisesTheManagedImportName(t *testing.T) {
	f := requirePair(t)

	// isManagedImport in the .ts file, restated: ^<export>-[a-z0-9]{15}$. The
	// id-shaped tail is what keeps a tenant's own "helpdesk-events-eu" editable,
	// so it is the half most likely to be wrong -- assert the real name against
	// it, not against a prefix check that could not tell the two apart.
	name := consoleExportName(t)
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `-[a-z0-9]{15}$`)

	got := f.imported.GetString("name")
	if !pattern.MatchString(got) {
		t.Errorf("the hook names the hub import %q, which isManagedImport() does not match (%s).\n"+
			"The console would treat a platform-provisioned import as an ordinary editable one.", got, pattern)
	}

	// Keyed by the immutable org id, deliberately -- that is what makes a
	// renamed org a reconcile rather than a delete-and-recreate.
	if want := name + "-" + f.tenant.Id; got != want {
		t.Errorf("import name %q is not <export>-<org id> (%q)", got, want)
	}
}

// The tenant's export shows up in the TENANT's Exports list; the hub import
// shows up in the OPERATOR's Imports list. Two different screens, and this is
// what says which record lands on which -- a swap would put the read-only
// banner in front of the wrong operator.
func TestTheManagedPairIsSplitAcrossTheTwoAccounts(t *testing.T) {
	f := requirePair(t)

	accountFor := func(org *core.Record, what string) *core.Record {
		rec, err := f.app.FindFirstRecordByFilter("nats_accounts",
			"organization = {:org}", map[string]any{"org": org.Id})
		if err != nil {
			t.Fatalf("%s NATS account: %v", what, err)
		}
		return rec
	}

	if got, want := f.exported.GetString("account_id"), accountFor(f.tenant, "tenant").Id; got != want {
		t.Errorf("export sits on account %q, want the TENANT's own account %q", got, want)
	}
	if got, want := f.imported.GetString("account_id"), accountFor(f.hub, "hub").Id; got != want {
		t.Errorf("import sits on account %q, want the OPERATOR HUB account %q", got, want)
	}

	// The org token on the remapped subject is the CODE, not the id (ADR 0002).
	// Derived from the name here, so this covers RegisterOrgCode too.
	wantLocal := "helpdesk." + f.tenant.GetString("code") + ".>"
	if got := f.imported.GetString("local_subject"); got != wantLocal {
		t.Errorf("local_subject = %q, want %q", got, wantLocal)
	}
}

// The fields the console banner NAMES as reverting are the fields reconcile
// actually rewrites. Add one to exportFields/importFields and the banner is
// understating the damage; remove one and it is overstating it. Either way the
// text needs editing, and this is what says so.
func TestTheBannerNamesTheFieldsThatActuallyRevert(t *testing.T) {
	f := requirePair(t)

	// Reconciled on the export: subject, type, description (plus organization,
	// which is bookkeeping the banner has no reason to mention).
	if got := f.exported.GetString("subject"); got != "helpdesk.>" {
		t.Errorf("export subject = %q, want %q", got, "helpdesk.>")
	}
	if got := f.exported.GetString("type"); got != "stream" {
		t.Errorf("export type = %q, want stream", got)
	}
	if f.exported.GetString("description") == "" {
		t.Error("export description is empty, but reconcile sets it -- the banner claims it reverts")
	}

	// Reconciled on the import: those three plus account and local_subject.
	if got := f.imported.GetString("type"); got != "stream" {
		t.Errorf("import type = %q, want stream", got)
	}
	if f.imported.GetString("account") == "" {
		t.Error("import account is empty, but reconcile sets it from the tenant account's public key")
	}
	if f.imported.GetString("local_subject") == "" {
		t.Error("import local_subject is empty, but reconcile sets it")
	}

	// The create-only half, which the banner deliberately does NOT claim
	// reverts: token_req is set once on create and left alone thereafter.
	if f.exported.GetBool("token_req") {
		t.Error("export token_req is true; the hook sets it false on create as a policy flag")
	}
}
