// Package hooks registers PocketBase hooks that live in this repo (as opposed
// to the library-supplied hooks from pb-tenancy, pb-nats, pb-nebula, pb-audit).
package hooks

import (
	"log"
	"sort"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/subjectresolver"
)

// RegisterThingTypeRoleSync wires hooks that recompute the content of the
// linked nats_role record whenever a thing_types record changes or one of the
// thing_type_operations records it links to changes.
//
// The platform owns the *derivation* — turning a Thing Type's contract
// (subject_prefix + linked operations) into NATS subject patterns. pb-nats
// continues to own the nats_role record itself, its signing, and everything
// downstream (nats_users, JWTs, creds).
//
// A Thing Type with no nats_role linked produces no permissions, which is the
// correct default for existing data.
func RegisterThingTypeRoleSync(app *pocketbase.PocketBase) {
	syncTT := func(e *core.RecordEvent) error {
		if err := syncThingTypeRole(e.App, e.Record); err != nil {
			log.Printf("❌ thing_type role sync failed for %s: %v", e.Record.Id, err)
		}
		return e.Next()
	}
	app.OnRecordAfterCreateSuccess("thing_types").BindFunc(syncTT)
	app.OnRecordAfterUpdateSuccess("thing_types").BindFunc(syncTT)

	syncOp := func(e *core.RecordEvent) error {
		if err := propagateOperation(e.App, e.Record); err != nil {
			log.Printf("❌ thing_type_operation propagation failed for %s: %v", e.Record.Id, err)
		}
		return e.Next()
	}
	app.OnRecordAfterCreateSuccess("thing_type_operations").BindFunc(syncOp)
	app.OnRecordAfterUpdateSuccess("thing_type_operations").BindFunc(syncOp)
}

// syncThingTypeRole writes the publish/subscribe permission patterns derived
// from the Thing Type's operations onto its linked nats_role record. If no
// role is linked it is a no-op.
func syncThingTypeRole(app core.App, tt *core.Record) error {
	roleID := tt.GetString("nats_role")
	if roleID == "" {
		return nil
	}
	role, err := app.FindRecordById("nats_roles", roleID)
	if err != nil {
		return err
	}

	prefix := tt.GetString("subject_prefix")
	ttCode := tt.GetString("code")
	opIDs := tt.GetStringSlice("operations")

	ctx := subjectresolver.RolePatternContext{ThingTypeCode: ttCode}

	var pub, sub []string
	allowResponse := false

	for _, opID := range opIDs {
		op, err := app.FindRecordById("thing_type_operations", opID)
		if err != nil {
			continue
		}
		subj := subjectresolver.ResolveRolePattern(
			subjectresolver.Join(prefix, op.GetString("subject_suffix")),
			ctx,
		)
		switch op.GetString("capability") {
		case "publish", "request":
			// request: Thing publishes to the request subject.
			pub = append(pub, subj)
		case "subscribe":
			sub = append(sub, subj)
		case "reply":
			// reply: Thing subscribes to the request subject; NATS' allow_response
			// governs the reply publish permission via the inbox.
			sub = append(sub, subj)
			allowResponse = true
		}
	}

	role.Set("publish_permissions", uniqueSorted(pub))
	role.Set("subscribe_permissions", uniqueSorted(sub))
	if allowResponse {
		role.Set("allow_response", true)
	}
	return app.Save(role)
}

// propagateOperation finds every thing_type that references the given operation
// and re-syncs each one's linked nats_role. Covers the case where editing a
// shared operation must reflect on all Thing Types that use it.
func propagateOperation(app core.App, op *core.Record) error {
	tts, err := app.FindRecordsByFilter(
		"thing_types",
		"operations ?= {:op}",
		"",
		0,
		0,
		dbx.Params{"op": op.Id},
	)
	if err != nil {
		return err
	}
	for _, tt := range tts {
		if err := syncThingTypeRole(app, tt); err != nil {
			log.Printf("❌ sync for thing_type %s: %v", tt.Id, err)
		}
	}
	return nil
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
