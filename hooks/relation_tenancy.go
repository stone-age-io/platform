package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// orgFieldName is the column that makes a collection tenant-scoped. Every API
// rule in schema.json keys off it, so its presence is also the most reliable
// signal that a record belongs to somebody.
const orgFieldName = "organization"

// RegisterRelationTenancy refuses a relation that points into another
// organization's records.
//
// WHY THIS IS NOT AN API RULE. `nats_users.createRule` pins the new record's own
// organization (`@request.body.organization = @request.auth.current_organization`)
// and says nothing about `account_id`. A rule cannot easily say more: PocketBase
// can traverse a relation on a STORED field, but `@request.body.account_id` is
// the raw submitted string, with no target to traverse into. So the check has to
// live in code.
//
// WHY IT MATTERS. pb-nats resolves the signing account straight off that field —
// `generateUserJWT` in internal/sync/manager.go does
// `FindRecordById(AccountCollectionName, record.GetString("account_id"))` and
// signs with whatever comes back. A tenant owner/admin could therefore create a
// nats_users row carrying `organization = <their org>` and `account_id = <another
// tenant's account>`, and receive a valid NATS credential inside that tenant's
// account. The only thing standing in the way today is that `nats_accounts`
// list-scoping hides the id, and id secrecy is not an authorization boundary.
//
// This is the same check hooks/thing_routes.go already makes by hand for the
// nats_user/nebula_host it links on POST /api/org/things — "without that second
// check the route would be a cross-tenant credential-theft path". It was simply
// never made for the CRUD endpoints, or for any of the other eighteen relations
// with the same shape.
//
// WHY IT IS DERIVED RATHER THAN LISTED. There are nineteen relations between
// org-scoped collections, and the rule for every one of them is the same
// sentence. A hand-kept table is one more parallel list to fall out of step with
// schema.json, which is the failure mode behind most of the authorization bugs
// this codebase has already paid for. Instead the guard reads the collection the
// record actually belongs to: if that collection has an `organization` field, and
// a relation field on it points at another collection that also has one, the two
// records must agree. Add such a relation tomorrow and it is covered without
// anyone remembering to come back here.
//
// The cross-org linkage that IS legitimate does not travel through a relation
// field. A managed org's events reach the operator hub through an exported
// stream and a subject remapped into the hub account's own JWT
// (hooks/managed_org_exports.go); both the export and the import record sit in
// the same organization as the account they name. Provenance is subject-based
// and unforgeable, which is exactly why no relation has to cross a tenant line.
//
// This binds the model hooks, so it covers a route doing app.Save() as well as
// the REST endpoints, and it applies to superusers too. That last part is
// deliberate: a relation spanning two tenants is corrupt data whoever wrote it,
// and unlike the immutable-code guard there is no dashboard workflow that needs
// the exemption.
func RegisterRelationTenancy(app *pocketbase.PocketBase) {
	guard := func(e *core.RecordEvent) error {
		if err := checkRelationTenancy(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	}

	// No collection filter: the guard decides for itself from the schema, and a
	// list of names here would be the parallel list the derivation exists to
	// avoid. Records in collections with no `organization` field cost one
	// in-memory field lookup.
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

		// On an update, ids already present before this write have been checked
		// once and cost a query to check again. On a create Original() is empty,
		// so everything set is examined.
		previous := make(map[string]struct{})
		for _, id := range record.Original().GetStringSlice(rel.Name) {
			previous[id] = struct{}{}
		}

		// GetStringSlice covers both cardinalities, so maxSelect never has to be
		// consulted here.
		for _, id := range record.GetStringSlice(rel.Name) {
			if id == "" {
				continue
			}
			if _, unchanged := previous[id]; unchanged {
				continue
			}

			related, err := app.FindRecordById(target.Name, id)
			if err != nil {
				// A dangling id is PocketBase's own relation validation to report;
				// answering it here would just produce a second, worse message.
				continue
			}

			if related.GetString(orgFieldName) != recordOrg {
				// apis.NewBadRequestError, not fmt.Errorf: a plain error from a hook
				// reaches the client as "Failed to create record." and the operator
				// never learns which field was wrong.
				return apis.NewBadRequestError(
					"The "+rel.Name+" field must reference a record in the same organization.",
					nil,
				)
			}
		}
	}

	return nil
}
