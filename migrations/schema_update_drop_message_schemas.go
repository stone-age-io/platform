package migrations

import (
	"fmt"
	"log"
	"slices"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Drops the half of the Thing Type "contract layer" that nothing consumed:
//
//   - the message_schemas collection, whose documents were never validated
//     against anything and never read outside one widget's payload form
//   - thing_type_operations.schema, the relation into it
//   - thing_types.capabilities, a hand-maintained union of its operations'
//     capabilities. Nothing sorted, filtered, or derived anything from it, and
//     it could disagree with the operations it summarised without any warning.
//     The per-operation `capability` field stays: it is what distinguishes a
//     request from a reply on the same subject suffix.
//   - thing_types.nats_role, a relation read by no Go and no TypeScript in the
//     tree. It was added alongside `operations` as the intended bridge from a
//     contract to a NATS permission set; that bridge was never built.
//
// WHY THERE IS NO schema.json RE-IMPORT HERE. Every other schema_update_*.go in
// this package is "re-import schema.json", and a re-import cannot express a
// removal. initial_schema.go calls ImportCollectionsByMarshaledJSON with
// deleteMissing=false, and in that mode core.ImportCollections walks the LIVE
// collection and re-adds every field the import did not carry BY ID
// (core/collection_import.go), so deleting a field from schema.json and
// re-importing logs its ✅ and changes nothing. Worse, an import placed AFTER
// the removals below would resurrect all three fields in the same run.
// TestReimportCannotRemoveAField pins that behaviour against the real importer;
// if it ever starts failing, this file can be reconsidered.
//
// Nothing here changes a collection rule, which is the other thing a re-import
// is for — so there is nothing for one to do.
//
// The ORDER is load-bearing: PocketBase refuses to delete a collection that
// still has relation references pointing at it ("failed to delete due to
// existing relation references", core/collection_model.go), so
// thing_type_operations.schema has to go before message_schemas does.
//
// Every step is a no-op when its target is already absent. On a fresh database
// initial_schema.go imports the already-thinned schema.json, so none of these
// exist by the time this runs.
func init() {
	m.Register(DropContractLayer, nil)
}

// DropContractLayer is the migration body.
//
// Exported, and named rather than inlined into m.Register, so a test can run it
// against a database that actually HAS the old schema. The migration set alone
// only ever exercises the no-op path: a fresh database imports the thinned
// schema.json first, so by the time this runs there is nothing left to remove,
// and a green migration would prove nothing about the removal itself. The test
// lives in package migrations_test because internal/testutil imports this
// package, so an in-package test could not reach the harness.
func DropContractLayer(app core.App) error {
	// 1. The relation into message_schemas, before the collection it points at.
	if err := dropField(app, "thing_type_operations", "relation_tto_schema"); err != nil {
		return err
	}

	// 2. The collection itself.
	if col, err := app.FindCollectionByNameOrId("message_schemas"); err == nil && col != nil {
		if err := app.Delete(col); err != nil {
			return fmt.Errorf("delete message_schemas: %w", err)
		}
		log.Println("✅ Dropped the message_schemas collection")
	}

	// 3. The two dead fields on thing_types.
	if err := dropField(app, "thing_types", "select490417661"); err != nil { // capabilities
		return err
	}
	if err := dropField(app, "thing_types", "relation_tt_nats_role"); err != nil { // nats_role
		return err
	}

	// 4. Stale values in leaf_nodes.synced_collections.
	//
	// internal/leafsync intersects this list with its own hard allowlist, so a
	// leftover "message_schemas" was never going to sync anything — but the
	// leaf node detail view renders these values verbatim, and a badge naming
	// a collection that no longer exists is a screen telling the operator
	// something untrue about what their edge box mirrors.
	if err := pruneSyncedCollections(app, "message_schemas"); err != nil {
		return err
	}

	return nil
}

// dropField removes one field by ID and persists the collection. Keyed on the
// id rather than the name for the same reason the importer is: a name matches
// whatever is live under it, where an id matches the definition this migration
// was written against.
func dropField(app core.App, collection, fieldID string) error {
	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil || col == nil {
		return nil // collection is gone or was never created; nothing to drop
	}

	f := col.Fields.GetById(fieldID)
	if f == nil {
		return nil
	}

	col.Fields.RemoveById(fieldID)
	if err := app.Save(col); err != nil {
		return fmt.Errorf("drop %s.%s: %w", collection, f.GetName(), err)
	}

	log.Printf("✅ Dropped %s.%s", collection, f.GetName())
	return nil
}

// pruneSyncedCollections removes one value from every leaf node's
// synced_collections list.
func pruneSyncedCollections(app core.App, value string) error {
	nodes, err := app.FindAllRecords("leaf_nodes")
	if err != nil {
		return nil // collection absent on a database that predates leaf nodes
	}

	pruned := 0
	for _, n := range nodes {
		current := n.GetStringSlice("synced_collections")
		if !slices.Contains(current, value) {
			continue
		}
		next := slices.DeleteFunc(slices.Clone(current), func(s string) bool { return s == value })
		n.Set("synced_collections", next)
		if err := app.Save(n); err != nil {
			return fmt.Errorf("prune %q from leaf node %s: %w", value, n.Id, err)
		}
		pruned++
	}

	if pruned > 0 {
		log.Printf("✅ Removed %q from synced_collections on %d leaf node(s)", value, pruned)
	}
	return nil
}
