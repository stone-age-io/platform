package hooks

import (
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// ActivityCollection is the tenant-facing feed of who changed what.
const ActivityCollection = "activity"

// The actions a feed entry can carry: the `action` select values.
const (
	ActionCreated     = "created"
	ActionUpdated     = "updated"
	ActionDeleted     = "deleted"
	ActionProvisioned = "provisioned"
)

// Actor types. The PocketBase dashboard at /_ drives the same record endpoints,
// so the caller is not always a `users` record.
const (
	ActorUser      = "user"
	ActorSuperuser = "superuser"
)

// activityCollections are the collections whose writes reach the feed, and the
// product noun each is reported as.
//
// This list is the safety argument. An entry must be visible to exactly those
// who could read the record it describes, and the feed's read rule is
// org-scoped with no role branch, so it may only cover collections whose own
// read rule is the same. `memberships`, `invites`, `nats_roles` and
// `nebula_networks` stop at owner/admin and must stay out. Adding an entry is an
// authorization change; TestActivityCollectionsAreOrgReadable pins it.
//
// Hand-kept rather than derived from the schema (unlike
// hooks/relation_tenancy.go): which writes a tenant cares about is a product
// judgement no schema fact expresses.
var activityCollections = map[string]string{
	"things":                "thing",
	"locations":             "location",
	"thing_types":           "thing type",
	"location_types":        "location type",
	"thing_type_operations": "thing operation",
}

// ActivityCollectionNames returns the collections the feed covers. main.go uses
// it to exclude the feed from audit snapshots.
func ActivityCollectionNames() []string {
	names := make([]string, 0, len(activityCollections))
	for name := range activityCollections {
		names = append(names, name)
	}
	return names
}

// RegisterActivity records tenant-visible console writes into the feed.
//
// Request hooks, because they are the only layer that knows the actor.
// `core.RecordEvent` carries a Context, but nothing in PocketBase puts auth into
// it (which is also why pb-audit passes nil request info on its success hooks).
//
// The snapshot is taken before e.Next(), so the entry describes what the
// request asked for, not what server-side hooks (RefreshTokenKey, pb-nats and
// pb-nebula generated fields) wrote on the way through.
//
// The entry is saved through e.App, not the root app. For plain CRUD a nil
// e.Next() means committed. Under the batch API (off by default, unused by the
// console) e.App is the transaction, so the row commits or rolls back with its
// write; the root app would deadlock waiting on SQLite's only write connection.
//
// The entry lands after the response, so a client that saves and reloads the
// feed may miss its own entry for a beat, and a test must wait. Not worth a
// transaction to remove.
//
// Failures are logged, never fatal, unlike hooks/org_provisioning.go: this is an
// observation, and it must not cost the user their write.
func RegisterActivity(app *pocketbase.PocketBase) {
	names := ActivityCollectionNames()

	app.OnRecordCreateRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		entry := describe(e, ActionCreated)
		if err := e.Next(); err != nil {
			return err
		}
		// The one field captured after the write: a new record has no id until
		// the save assigns it.
		entry.ResourceID = e.Record.Id
		entry.save(e.App)
		return nil
	})

	app.OnRecordUpdateRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		entry := describe(e, ActionUpdated)
		if err := e.Next(); err != nil {
			return err
		}
		entry.save(e.App)
		return nil
	})

	app.OnRecordDeleteRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		entry := describe(e, ActionDeleted)
		if err := e.Next(); err != nil {
			return err
		}
		entry.save(e.App)
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
		// Empty on a create; the create hook fills it in after the save.
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

// save writes the row, best-effort. Pass the request's app (e.App), never the
// root app: under the batch API only e.App can write.
func (ent entry) save(app core.App) {
	// A guard: every covered collection requires an organization.
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

// RecordActivity writes one feed entry from a route. A route that writes with
// app.Save() bypasses the request hooks, so it must log itself. Today that is
// only POST /api/org/things; the other org routes touch collections the feed
// must not cover.
//
// The actor comes from the request event, never an argument, so a caller cannot
// attribute an entry to someone else.
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

// recordLabel is the human name of the record an entry describes. A snapshot,
// not a join: a log must not change when a record is renamed, and must still
// read after it is deleted. All five covered collections carry `name`.
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

// actorLabel is the human name of whoever acted, snapshotted like recordLabel.
// `actor` is a plain id so that deleting a user does not rewrite every row.
func actorLabel(auth *core.Record) string {
	return firstNonEmpty(auth.GetString("name"), auth.Email(), auth.Id)
}
