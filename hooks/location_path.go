package hooks

import (
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// locations.path -- ADR 0004 in platform-docs.
//
// A path is the codes from the root down, `/` between them and at both ends:
// `/KC/BD-3/RM-204/`. It turns "everything under BD-3", which would otherwise
// need recursion, into one substring match (`location.path ~ '/BD-3/'`, or
// `location_path=~".*/BD-3/.*"` in PromQL). The end slashes stop `/BD-3/`
// matching inside `/BD-30/`; codes cannot contain `/`.
//
// This hook is the only writer: every save recomputes the path and overwrites
// what the client sent. Deliberately no `:changed = false` rule term: a form
// loaded before an ancestor moved would echo a stale path and 404.
//
// A path changes when the location moves, or when a pre-ADR 0003 location gets
// its first code (see pathSegment). The subtree is rewritten in the same
// transaction by one UPDATE.
//
// A deleted parent needs no delete hook: PocketBase unsets the relation on each
// child with SaveNoValidate inside the delete's transaction, which fires the
// update hook, so each child becomes a root. TestLocationPath pins this, since
// it is PocketBase's behaviour.

const locationPathField = "path"

// pathSegment is the location's own segment: its code, or its id when it has no
// code (only pre-ADR 0003 locations lack one). The id keeps two code-less
// siblings from sharing a path, and so from moving with each other.
func pathSegment(rec *core.Record) string {
	if code := rec.GetString("code"); code != "" {
		return code
	}
	return rec.Id
}

// computeLocationPath returns the path rec should hold: its parent's stored path
// plus its own segment, or just its own segment for a root. A parent in another
// organization is refused here too (not only by RegisterRelationTenancy): the
// path would leak that tenant's codes.
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
		// Only a location the migration could not place (a looped chain).
		return "", apis.NewBadRequestError("The parent location has no path yet. Save the parent first.", nil)
	}
	return parentPath + pathSegment(rec) + "/", nil
}

// RegisterLocationPath binds the path computation. Register it AFTER
// RegisterCodes, so the path is built from the generated code.
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
		// Read from the table, not e.Record.Original(), which is blank for a
		// record built with core.NewRecord and saved twice.
		var oldPath string
		if err := e.App.DB().NewQuery("SELECT [[path]] FROM {{locations}} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"id": e.Record.Id}).Row(&oldPath); err != nil {
			return err
		}
		newPath, err := computeLocationPath(e.App, e.Record)
		if err != nil {
			return err
		}

		// A new path under the old one means the new parent is a descendant.
		// The UI greys those out; this covers every other client.
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
			// substr, NOT LIKE: codes may contain `_`, a LIKE wildcard.
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
