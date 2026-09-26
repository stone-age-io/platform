package hooks

import (
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// locations.path -- ADR 0004 in platform-docs.
//
// A path is the codes from the root down to the location itself, `/` between
// them and at both ends: `/KC/BD-3/RM-204/`. It exists because `parent` is a
// single relation and "everything under BD-3" needs recursion, which neither a
// PocketBase filter nor PromQL has. With a path it is one substring match:
// `location.path ~ '/BD-3/'` here, `location_path=~".*/BD-3/.*"` in a dashboard.
//
// The slashes at both ends are what make a segment match whole: `/BD-3/` can
// never match inside `/BD-30/`. Codes cannot contain `/` (CodePattern), so a
// segment is never ambiguous either.
//
// THIS HOOK IS THE ONLY WRITER. Every create and update recomputes the path
// from the parent's stored path and the location's own code, and whatever a
// client sent is overwritten. There is deliberately no update-rule term
// refusing `path` from clients: a form loaded before an ancestor moved would
// echo a stale path, and `:changed = false` would turn that save into a 404 for
// a field the client never meant to touch. Overwriting says the same thing --
// clients do not write paths -- without the false failure.
//
// A PATH CHANGES IN TWO CASES: the location moves under a different parent, or
// a location that predates ADR 0003 gets its first code (see pathSegment). Either
// way every location underneath moves with it, rewritten in the same
// transaction by one UPDATE that swaps the old prefix for the new.
//
// Deleting a parent is covered without a delete hook. PocketBase unsets a
// deleted record's id from every relation pointing at it and saves each
// referencing record with SaveNoValidate, inside the delete's transaction. That
// save fires OnRecordUpdate here, so each orphaned child becomes a root (`/CODE/`)
// and takes its own subtree with it. TestLocationPath pins this, because it is
// PocketBase's behaviour rather than ours.

const locationPathField = "path"

// pathSegment is the location's own segment: its code, or its id when it has no
// code. Only locations created before ADR 0003 can lack one. Falling back to the
// id rather than an empty segment keeps two code-less siblings from sharing a
// path, which would make the subtree rewrite for one of them move the other.
func pathSegment(rec *core.Record) string {
	if code := rec.GetString("code"); code != "" {
		return code
	}
	return rec.Id
}

// computeLocationPath returns the path rec should hold: its parent's stored path
// plus its own segment, or just its own segment for a root. A parent in another
// organization is refused here as well as by RegisterRelationTenancy, because a
// path spliced from another tenant's tree would leak that tenant's codes.
func computeLocationPath(app core.App, rec *core.Record) (string, error) {
	parentID := rec.GetString("parent")
	if parentID == "" {
		return "/" + pathSegment(rec) + "/", nil
	}
	if parentID == rec.Id {
		return "", apis.NewBadRequestError("A location cannot be its own parent.", nil)
	}
	parent, err := app.FindRecordById(locationsCollection, parentID)
	if err != nil || parent.GetString("organization") != rec.GetString("organization") {
		return "", apis.NewBadRequestError("The parent location is not in this organization.", nil)
	}
	parentPath := parent.GetString(locationPathField)
	if parentPath == "" {
		// Only a location the path migration could not place (a parent chain that
		// looped) has none. Building on it would give a path with no root.
		return "", apis.NewBadRequestError("The parent location has no path yet. Save the parent first.", nil)
	}
	return parentPath + pathSegment(rec) + "/", nil
}

// RegisterLocationPath binds the path computation. Register it AFTER
// RegisterCodes: a location created without a code gets one there, and the path
// has to be built from that code, not from the blank it arrived with.
func RegisterLocationPath(app *pocketbase.PocketBase) {
	app.OnRecordCreate(locationsCollection).BindFunc(func(e *core.RecordEvent) error {
		path, err := computeLocationPath(e.App, e.Record)
		if err != nil {
			return err
		}
		e.Record.Set(locationPathField, path)
		return e.Next()
	})

	app.OnRecordUpdate(locationsCollection).BindFunc(func(e *core.RecordEvent) error {
		// Read from the table, not e.Record.Original(): a record built with
		// core.NewRecord and saved twice has a blank Original, and trusting it
		// would skip the subtree rewrite without a word.
		var oldPath string
		if err := e.App.DB().NewQuery("SELECT [[path]] FROM {{locations}} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"id": e.Record.Id}).Row(&oldPath); err != nil {
			return err
		}
		newPath, err := computeLocationPath(e.App, e.Record)
		if err != nil {
			return err
		}

		// A new parent whose path starts with this location's own path is this
		// location or something under it, and taking it as a parent would make
		// the location its own ancestor. The UI greys those rows out; this is
		// the check every other client gets.
		if oldPath != "" && e.Record.GetString("parent") != "" && strings.HasPrefix(newPath, oldPath) && newPath != oldPath {
			return apis.NewBadRequestError("A location cannot be moved under one of its own descendants.", nil)
		}

		e.Record.Set(locationPathField, newPath)
		if newPath == oldPath || oldPath == "" {
			return e.Next()
		}

		return e.App.RunInTransaction(func(txApp core.App) error {
			e.App = txApp
			if err := e.Next(); err != nil {
				return err
			}
			// substr, NOT LIKE: codes may contain `_`, which LIKE reads as a
			// wildcard, so `/KC/A_1/%` would also rewrite `/KC/AB1/`.
			_, err := txApp.DB().NewQuery(
				"UPDATE {{locations}} SET [[path]] = {:new} || substr([[path]], length({:old}) + 1) " +
					"WHERE [[organization]] = {:org} AND [[id]] != {:id} " +
					"AND substr([[path]], 1, length({:old})) = {:old}",
			).Bind(dbx.Params{
				"new": newPath,
				"old": oldPath,
				"org": e.Record.GetString("organization"),
				"id":  e.Record.Id,
			}).Execute()
			return err
		})
	})
}
