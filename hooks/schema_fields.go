package hooks

import "github.com/pocketbase/pocketbase/core"

// CollectionFields pairs a collection with fields a caller expects on it.
type CollectionFields struct {
	Collection string
	Fields     []string
}

// PlatformSchemaFields names the fields that exist only once schema.json has
// been imported.
//
// WHY THIS LIST IS SHARED RATHER THAN WRITTEN TWICE. Two callers ask the same
// question for the same reason: PocketBase discards a write to a field that
// does not exist, silently and without an error, so a binary running against a
// database that never got `migrate up` will set a platform flag and report
// success having written nothing. `bootstrap` refuses to run in that state and
// the `schema` readiness check reports it. Both were carrying their own copy of
// the list AND their own copy of the walk below, which is the parallel-list
// failure this codebase has paid for elsewhere: a field added to schema.json
// and to one of the two lists leaves the other silently checking less than it
// claims to.
//
// The collection names are parameters because config.yaml can rename them.
//
// internal/demoseed has a similar preflight and is deliberately NOT routed
// through here: it checks a different list (the inventory `code` columns its
// fixtures are built on) for a different reason, so sharing would buy a common
// shape and no common fact.
func PlatformSchemaFields(orgCollection, membershipCollection string) []CollectionFields {
	return []CollectionFields{
		{"users", []string{"is_operator"}},
		{orgCollection, []string{"is_system_org", "is_operator_org"}},
		{membershipCollection, []string{"role"}},
	}
}

// MissingFields returns "collection.field" for each expected field that is not
// present. A missing collection reports all of its fields, which reads better
// than a separate "collection not found" case.
func MissingFields(app core.App, want []CollectionFields) []string {
	var missing []string
	for _, cf := range want {
		col, err := app.FindCollectionByNameOrId(cf.Collection)
		if err != nil || col == nil {
			for _, f := range cf.Fields {
				missing = append(missing, cf.Collection+"."+f)
			}
			continue
		}
		for _, f := range cf.Fields {
			if col.Fields.GetByName(f) == nil {
				missing = append(missing, cf.Collection+"."+f)
			}
		}
	}
	return missing
}
