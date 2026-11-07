package hooks

import (
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// orgFieldName is the column that makes a collection tenant-scoped. Every API
// rule in schema.json keys off it.
const orgFieldName = "organization"

// RegisterRelationTenancy refuses a relation that points into another
// organization's records.
//
// It is not an API rule because no rule can express it: PocketBase traverses a
// relation on a STORED field, but `@request.body.account_id` is a raw submitted
// string with no target to traverse into.
//
// It matters because pb-nats signs a user JWT with whatever `account_id` names
// (generateUserJWT in internal/sync/manager.go), so an owner could otherwise
// mint a credential inside another tenant's account. List-scoping hides the
// id, but id secrecy is not an authorization boundary.
//
// It is DERIVED from the schema rather than listing the nineteen relations: if
// a collection has an `organization` field and a relation on it points at
// another collection that has one, the two records must agree. A hand-kept
// list would drift from schema.json; a new relation is covered automatically.
// Legitimate cross-org linkage (managed exports, hooks/managed_org_exports.go)
// is subject-based and travels through no relation field.
//
// It binds the MODEL hooks, so it covers app.Save() routes as well as REST, and
// it applies to superusers: a cross-tenant relation is corrupt data whoever
// wrote it, and no dashboard workflow needs an exemption.
func RegisterRelationTenancy(app *pocketbase.PocketBase) {
	guard := func(e *core.RecordEvent) error {
		if err := checkRelationTenancy(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	}

	// No collection filter: the guard decides from the schema. A collection with
	// no `organization` field costs one in-memory field lookup.
	app.OnRecordCreate().BindFunc(guard)
	app.OnRecordUpdate().BindFunc(guard)
}

// checkRelationTenancy compares the record's organization against the
// organization of everything it points at.
func checkRelationTenancy(app core.App, record *core.Record) error {
	collection := record.Collection()
	if collection.Fields.GetByName(orgFieldName) == nil {
		return nil
	}

	recordOrg := record.GetString(orgFieldName)

	// When the record's OWN organization is moving, nothing on it has been
	// checked against the new one, so the unchanged-id shortcut below is off.
	// Otherwise a record could move tenant keeping every relation it held, the
	// signing account included. The record API forbids the move
	// (`@request.body.organization:changed = false`); a superuser in the
	// dashboard and an app.Save() route do not.
	orgMoved := record.Original().GetString(orgFieldName) != recordOrg

	for _, field := range collection.Fields {
		rel, ok := field.(*core.RelationField)
		if !ok || rel.Name == orgFieldName {
			continue
		}

		target, err := app.FindCollectionByNameOrId(rel.CollectionId)
		if err != nil || target.Fields.GetByName(orgFieldName) == nil {
			// Points at something unowned (users, for instance). Nothing to compare.
			continue
		}

		// On an update, ids present before this write were already checked. On a
		// create Original() is empty, so everything is examined.
		previous := make(map[string]struct{})
		if !orgMoved {
			for _, id := range record.Original().GetStringSlice(rel.Name) {
				previous[id] = struct{}{}
			}
		}

		// GetStringSlice covers both cardinalities.
		for _, id := range record.GetStringSlice(rel.Name) {
			if id == "" {
				continue
			}
			if _, unchanged := previous[id]; unchanged {
				continue
			}

			related, err := app.FindRecordById(target.Name, id)
			if err != nil {
				// A dangling id is PocketBase's own relation validation to report.
				continue
			}

			if related.GetString(orgFieldName) != recordOrg {
				// apis.NewBadRequestError, not fmt.Errorf: a plain hook error
				// reaches the client as "Failed to create record.".
				return apis.NewBadRequestError(
					"The "+rel.Name+" field must reference a record in the same organization.",
					nil,
				)
			}
		}
	}

	return nil
}
