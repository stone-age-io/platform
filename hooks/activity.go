package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ActivityCollection is the tenant-facing feed of who changed what.
const ActivityCollection = "activity"

// The actions a feed entry can carry. They are the `action` select values, so
// they are stable strings rather than an enum.
const (
	ActionCreated     = "created"
	ActionUpdated     = "updated"
	ActionDeleted     = "deleted"
	ActionProvisioned = "provisioned"
)

// Actor types. `superuser` exists because superusers BYPASS API rules and the
// PocketBase dashboard at /_ drives the same record endpoints, so the caller on
// one of these writes is not always a `users` record.
const (
	ActorUser      = "user"
	ActorSuperuser = "superuser"
)

// activityCollections are the collections whose writes reach the feed, and the
// product noun each is reported as.
//
// THIS LIST IS THE SAFETY ARGUMENT, not a convenience. The invariant is:
//
//	an activity entry is visible to exactly those who could read the record
//	it describes.
//
// A flat collection that mirrors other collections inherits none of their
// scoping -- which is what audit_logs taught this codebase expensively. The
// feed's own read rule is org-scoped with no role branch, so it may only
// describe records whose OWN read rule is org-scoped with no role branch.
// Exactly five collections qualify. `memberships`, `invites`, `nats_roles` and
// `nebula_networks` stop at owner/admin, so putting them here would tell a
// member that an invitation was sent and to whom -- a record they cannot read.
//
// Adding an entry is therefore an authorization change, not a feature toggle.
// TestActivityCollectionsAreOrgReadable reads schema.json and fails if any
// collection here has a role branch in its listRule.
//
// A hand-kept list rather than a derivation from the schema, deliberately, and
// the opposite of hooks/relation_tenancy.go: there the rule is the same sentence
// for every relation and a list would be the thing that drifts, while here
// "which writes a tenant cares about" is a product judgement that no schema
// fact expresses.
var activityCollections = map[string]string{
	"things":                "thing",
	"locations":             "location",
	"thing_types":           "thing type",
	"location_types":        "location type",
	"thing_type_operations": "thing operation",
}

// ActivityCollectionNames returns the collections the feed covers. Exported for
// main.go, which must exclude the feed from audit snapshots, and for the test
// that pins the invariant above.
func ActivityCollectionNames() []string {
	names := make([]string, 0, len(activityCollections))
	for name := range activityCollections {
		names = append(names, name)
	}
	return names
}

// RegisterActivity records tenant-visible console writes into the feed.
//
// WHY REQUEST HOOKS. The actor is the whole problem, and request hooks are the
// only layer that knows it. `core.RecordEvent` -- what the model hooks deliver
// -- carries a Context, but nothing in PocketBase ever puts auth into it; the
// only context.WithValue calls in the framework are in its own tests. That is
// also why pb-audit passes nil for request info on its success hooks.
// `core.RecordRequestEvent` embeds *core.RequestEvent, which has Auth.
//
// WHY THE SNAPSHOT IS TAKEN BEFORE e.Next(). The entry should describe what the
// request asked for, not what the request plus every server-side hook did.
// hooks/active_flag.go calls RefreshTokenKey() before its save, and pb-nats and
// pb-nebula both write generated fields back on the way through -- all of which
// would be attributed to the person who clicked. Capturing first is also the
// rule CLAUDE.md already states for exactly this reason: take the "before"
// state immediately before the action it belongs to.
//
// WHAT e.Next() RETURNING NIL MEANS. For plain CRUD it means committed: with no
// enclosing transaction core.BaseApp.create/update take the autocommit path, and
// delete commits its own transaction before returning. It is NOT guaranteed
// under the batch API (apis/batch.go drives these same handlers inside
// RunInTransaction), which is disabled by default and unused by the console. The
// cost if someone enables it is a phantom row in an observation log, which is
// not worth a transaction dance to prevent -- but it is worth writing down.
//
// THE ENTRY LANDS AFTER THE RESPONSE. e.Next() writes the HTTP response as well
// as performing the save, so a client can receive its 200 before the feed row
// exists. A console that saves and immediately reloads the feed may not see its
// own entry for a beat. That is inherent to logging after the fact and is not
// worth a transaction to remove -- but it is why a test that reads the feed
// straight after a write needs to wait.
//
// FAILURES ARE LOGGED, NEVER FATAL. The opposite of hooks/org_provisioning.go,
// and deliberately: provisioning is an INVARIANT whose failure leaves the tenant
// broken, while this is an OBSERVATION whose failure must not cost the user
// their write.
func RegisterActivity(app *pocketbase.PocketBase) {
	names := ActivityCollectionNames()

	app.OnRecordCreateRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		entry := describe(e, ActionCreated)
		if err := e.Next(); err != nil {
			return err
		}
		// The id is the ONE field that cannot be captured before the write: a
		// new record has no id until PocketBase assigns it during the save, so
		// describe() saw an empty string. Everything else is still the
		// pre-write snapshot, which is the point.
		entry.ResourceID = e.Record.Id
		entry.save(app)
		return nil
	})

	app.OnRecordUpdateRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		entry := describe(e, ActionUpdated)
		if err := e.Next(); err != nil {
			return err
		}
		entry.save(app)
		return nil
	})

	app.OnRecordDeleteRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		entry := describe(e, ActionDeleted)
		if err := e.Next(); err != nil {
			return err
		}
		entry.save(app)
		return nil
	})
}

// entry is one feed row, captured before the write and saved after it.
type entry struct {
	Organization  string
	Actor         string
	ActorType     string
	ActorLabel    string
	Action        string
	Resource      string
	ResourceID    string
	ResourceLabel string
}

// describe reads everything the entry needs off the request, before the write.
func describe(e *core.RecordRequestEvent, action string) entry {
	noun := activityCollections[e.Record.Collection().Name]
	if noun == "" {
		noun = e.Record.Collection().Name
	}

	ent := entry{
		Organization: e.Record.GetString(orgFieldName),
		Action:       action,
		Resource:     noun,
		// Empty on a create -- PocketBase assigns the id during the save, so the
		// create hook overwrites this once e.Next() has returned. Update and
		// delete both have it already.
		ResourceID:    e.Record.Id,
		ResourceLabel: recordLabel(e.Record),
	}

	if e.Auth != nil {
		ent.Actor = e.Auth.Id
		ent.ActorType = ActorUser
		if e.Auth.Collection().Name != "users" {
			ent.ActorType = ActorSuperuser
		}
		ent.ActorLabel = actorLabel(e.Auth)
	}

	return ent
}

// save writes the row, best-effort.
//
// It takes the root app rather than the request's, so the write is not bound to
// a transaction that has already finished by the time this runs.
func (ent entry) save(app core.App) {
	// No organization means nothing to scope the row to, and the column is
	// required. Unreachable through these collections -- every one of them
	// requires an organization -- so this is a guard, not a case.
	if ent.Organization == "" {
		return
	}

	collection, err := app.FindCollectionByNameOrId(ActivityCollection)
	if err != nil {
		app.Logger().Warn("activity collection not found, feed entry dropped",
			"collection", ActivityCollection, "error", err)
		return
	}

	rec := core.NewRecord(collection)
	rec.Set("organization", ent.Organization)
	rec.Set("actor", ent.Actor)
	rec.Set("actor_type", ent.ActorType)
	rec.Set("actor_label", ent.ActorLabel)
	rec.Set("action", ent.Action)
	rec.Set("resource", ent.Resource)
	rec.Set("resource_id", ent.ResourceID)
	rec.Set("resource_label", ent.ResourceLabel)

	if err := app.Save(rec); err != nil {
		app.Logger().Warn("could not record activity",
			"action", ent.Action, "resource", ent.Resource,
			"resource_id", ent.ResourceID, "error", err)
	}
}

// RecordActivity writes one feed entry from a route.
//
// Routes that write with app.Save() bypass the request hooks entirely, so a
// route that creates something a tenant should see has to say so itself. Today
// that is POST /api/org/things; the other org routes touch collections the feed
// does not cover, and giving them a call would break the invariant rather than
// improve the feed.
//
// The actor comes from the request event, never from an argument, for the same
// reason the organization does: a caller cannot attribute an entry to someone
// else.
func RecordActivity(app core.App, re *core.RequestEvent, orgID, action, resource, resourceID, resourceLabel string) {
	ent := entry{
		Organization:  orgID,
		Action:        action,
		Resource:      resource,
		ResourceID:    resourceID,
		ResourceLabel: resourceLabel,
	}
	if re != nil && re.Auth != nil {
		ent.Actor = re.Auth.Id
		ent.ActorType = ActorUser
		if re.Auth.Collection().Name != "users" {
			ent.ActorType = ActorSuperuser
		}
		ent.ActorLabel = actorLabel(re.Auth)
	}
	ent.save(app)
}

// recordLabel is the human name of the record an entry describes.
//
// A SNAPSHOT, not a join. The feed is append-only, and a log whose lines change
// when somebody renames a record is not a log -- it also has to keep reading
// correctly after the record is deleted, which is precisely when a relation
// would give nothing back.
//
// All five covered collections carry `name`; all but thing_type_operations also
// carry `code`. One resolver covers them, which is a direct dividend of keeping
// the collection list narrow.
func recordLabel(record *core.Record) string {
	return firstNonEmpty(record.GetString("name"), record.GetString("code"), record.Id)
}

// firstNonEmpty returns the first non-blank argument, or "".
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// actorLabel is the human name of whoever acted, snapshotted for the same
// reason recordLabel is: the feed has to stay readable after the account is
// gone, and `actor` is a plain id precisely so that deleting a user does not
// rewrite every row that mentions them.
func actorLabel(auth *core.Record) string {
	return firstNonEmpty(auth.GetString("name"), auth.Email(), auth.Id)
}
