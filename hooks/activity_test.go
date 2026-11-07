package hooks_test

import (
	"errors"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/hooks"
	"platform/internal/testutil"
)

// The harness does record CRUD rather than serving HTTP, so these tests call
// hooks.RecordActivity with a *core.RequestEvent carrying an Auth (the route
// path). TestActivityRequestHooksRecordEachAction covers the binding in
// RegisterActivity. The actor is the one thing a model hook cannot supply, so it is what to
// assert.
func TestActivityRecordsTheActor(t *testing.T) {
	f := setupActivity(t)

	loc := f.newLocation(t, "Lobby")
	if err := f.app.Save(loc); err != nil {
		t.Fatalf("create location: %v", err)
	}

	// Simulate what the request hook does around a create.
	f.record(t, loc, hooks.ActionCreated)

	entry := f.latest(t)
	if got := entry.GetString("actor"); got != f.member.Id {
		t.Errorf("actor = %q, want the acting user %q", got, f.member.Id)
	}
	if got := entry.GetString("actor_type"); got != hooks.ActorUser {
		t.Errorf("actor_type = %q, want %q", got, hooks.ActorUser)
	}
	if got := entry.GetString("actor_label"); got != "Member Person" {
		t.Errorf("actor_label = %q, want the display name snapshotted at write time", got)
	}
	if got := entry.GetString("organization"); got != f.org.Id {
		t.Errorf("organization = %q, want %q -- it must come from the record, never the caller", got, f.org.Id)
	}
	if got := entry.GetString("resource"); got != "location" {
		t.Errorf("resource = %q, want the product noun %q", got, "location")
	}
	if got := entry.GetString("resource_label"); got != "Lobby" {
		t.Errorf("resource_label = %q, want %q", got, "Lobby")
	}
}

// The label is a snapshot and `actor` a plain id, not a relation, which would be
// blanked on user delete.
func TestActivitySurvivesTheActorBeingDeleted(t *testing.T) {
	f := setupActivity(t)

	loc := f.newLocation(t, "Server Room")
	if err := f.app.Save(loc); err != nil {
		t.Fatalf("create location: %v", err)
	}
	f.record(t, loc, hooks.ActionCreated)

	if err := f.app.Delete(f.member); err != nil {
		t.Fatalf("delete the acting user: %v", err)
	}

	entry := f.latest(t)
	if got := entry.GetString("actor_label"); got != "Member Person" {
		t.Errorf("actor_label = %q after the user was deleted; the snapshot is the point", got)
	}
	if got := entry.GetString("resource_label"); got != "Server Room" {
		t.Errorf("resource_label = %q, want it unchanged", got)
	}
}

// An observation must never cost the user their write (unlike
// hooks/org_provisioning.go).
func TestActivityFailureDoesNotBreakTheWrite(t *testing.T) {
	f := setupActivity(t)

	f.app.OnRecordCreate(hooks.ActivityCollection).BindFunc(func(e *core.RecordEvent) error {
		return errors.New("simulated activity outage")
	})

	loc := f.newLocation(t, "Warehouse")
	if err := f.app.Save(loc); err != nil {
		t.Fatalf("the location write must succeed even when the feed is broken: %v", err)
	}

	// The producer swallows its own failure.
	f.record(t, loc, hooks.ActionCreated)

	if _, err := f.app.FindRecordById("locations", loc.Id); err != nil {
		t.Errorf("the location should still exist: %v", err)
	}
	n, err := f.app.CountRecords(hooks.ActivityCollection)
	if err != nil {
		t.Fatalf("count activity: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no feed entry when the write fails, got %d", n)
	}
}

type activityFixture struct {
	app    core.App
	org    *core.Record
	member *core.Record
}

func setupActivity(t *testing.T) *activityFixture {
	t.Helper()
	app := testutil.SetupApp(t)

	newRec := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}

	owner := newRec("users")
	owner.Set("email", "activity-owner@example.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}

	org := newRec("organizations")
	org.Set("name", "Activity Co")
	org.Set("active", true)
	org.Set("owner", owner.Id)
	if err := app.Save(org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	member := newRec("users")
	member.Set("email", "activity-member@example.test")
	member.Set("password", "member-password-123")
	member.Set("name", "Member Person")
	member.Set("current_organization", org.Id)
	if err := app.Save(member); err != nil {
		t.Fatalf("create member: %v", err)
	}

	return &activityFixture{app: app, org: org, member: member}
}

func (f *activityFixture) newLocation(t *testing.T, name string) *core.Record {
	t.Helper()
	col, err := f.app.FindCollectionByNameOrId("locations")
	if err != nil {
		t.Fatalf("locations collection: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("name", name)
	rec.Set("organization", f.org.Id)
	return rec
}

// record drives the producer the way a request hook would, with an Auth set.
func (f *activityFixture) record(t *testing.T, rec *core.Record, action string) {
	t.Helper()
	re := &core.RequestEvent{}
	re.App = f.app
	re.Auth = f.member
	hooks.RecordActivity(f.app, re, rec.GetString("organization"), action,
		"location", rec.Id, rec.GetString("name"))
}

func (f *activityFixture) latest(t *testing.T) *core.Record {
	t.Helper()
	rec, err := f.app.FindFirstRecordByFilter(hooks.ActivityCollection,
		"organization = {:org}", dbx.Params{"org": f.org.Id})
	if err != nil {
		t.Fatalf("no activity entry was recorded: %v", err)
	}
	return rec
}

// The binding itself: RegisterActivity on the three record request hooks. The
// tests above call RecordActivity directly, so without this a binding that
// stopped firing, or a create entry missing the id assigned during the save,
// would pass them all.
func TestActivityRequestHooksRecordEachAction(t *testing.T) {
	f := setupActivity(t)
	app, ok := f.app.(*pocketbase.PocketBase)
	if !ok {
		t.Fatalf("setupActivity returned %T, want *pocketbase.PocketBase", f.app)
	}
	hooks.RegisterActivity(app)

	loc := f.newLocation(t, "Boiler Room")
	event := func() *core.RecordRequestEvent {
		e := &core.RecordRequestEvent{RequestEvent: &core.RequestEvent{App: app, Auth: f.member}, Record: loc}
		e.Collection = loc.Collection()
		return e
	}
	actions := []struct {
		action  string
		trigger func() error
	}{
		{hooks.ActionCreated, func() error {
			return app.OnRecordCreateRequest("locations").Trigger(event(), func(e *core.RecordRequestEvent) error {
				return e.App.Save(e.Record)
			})
		}},
		{hooks.ActionUpdated, func() error {
			loc.Set("name", "Plant Room")
			return app.OnRecordUpdateRequest("locations").Trigger(event(), func(e *core.RecordRequestEvent) error {
				return e.App.Save(e.Record)
			})
		}},
		{hooks.ActionDeleted, func() error {
			return app.OnRecordDeleteRequest("locations").Trigger(event(), func(e *core.RecordRequestEvent) error {
				return e.App.Delete(e.Record)
			})
		}},
	}

	for _, a := range actions {
		if err := a.trigger(); err != nil {
			t.Fatalf("%s: %v", a.action, err)
		}
		entry, err := app.FindFirstRecordByFilter(hooks.ActivityCollection,
			"organization = {:org} && action = {:a}", dbx.Params{"org": f.org.Id, "a": a.action})
		if err != nil {
			t.Fatalf("%s: no feed entry from the request hook: %v", a.action, err)
		}
		if got := entry.GetString("actor"); got != f.member.Id {
			t.Errorf("%s: actor = %q, want %q", a.action, got, f.member.Id)
		}
		if got := entry.GetString("resource_id"); got == "" || got != loc.Id {
			t.Errorf("%s: resource_id = %q, want the saved record's id %q", a.action, got, loc.Id)
		}
	}
}
