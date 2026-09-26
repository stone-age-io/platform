package migrations

import (
	"log"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_location_path adds locations.path (ADR 0004 in platform-docs)
// and fills it for the locations that already exist. hooks/location_path.go
// keeps it current from then on.
//
// The backfill writes with plain UPDATEs rather than app.Save, so no hook,
// realtime event or activity row fires for what is a computed column appearing,
// not an edit anyone made.
//
// A location whose parent chain loops back on itself, or leaves the
// organization, gets no path and is named in the log. Nothing is refused and
// nothing is guessed: the UI never offered a descendant as a parent, so a loop
// here was made by hand, and which member should be the root is the same kind of
// decision earlier migrations leave to a person. Make one member a root (or give
// it a correct parent), then save the rest once each, top down: the hook builds
// a path from the parent's, and refuses while the parent has none.
func init() {
	m.Register(ApplyLocationPath, nil)
}

// ApplyLocationPath is the migration body, exported for its test: a fresh
// database has no locations, so `migrate up` alone backfills nothing.
func ApplyLocationPath(app core.App) error {
	if len(SchemaJSON) == 0 {
		log.Println("⚠️ SchemaJSON is empty, skipping location path")
		return nil
	}
	if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
		return err
	}

	type loc struct {
		ID           string `db:"id"`
		Organization string `db:"organization"`
		Code         string `db:"code"`
		Parent       string `db:"parent"`
		Path         string `db:"path"`
	}
	var rows []loc
	if err := app.DB().NewQuery(
		"SELECT [[id]], [[organization]], [[code]], [[parent]], [[path]] FROM {{locations}}",
	).All(&rows); err != nil {
		return err
	}
	byID := make(map[string]loc, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	// pathOf walks up to the root. `seen` is the chain being walked, so a parent
	// already on it is a loop. The segment rule is hooks.pathSegment's: the code,
	// or the id for a location that predates generated codes.
	memo := map[string]string{}
	var pathOf func(id string, seen map[string]bool) (string, bool)
	pathOf = func(id string, seen map[string]bool) (string, bool) {
		if p, ok := memo[id]; ok {
			return p, p != ""
		}
		l := byID[id]
		segment := l.Code
		if segment == "" {
			segment = l.ID
		}
		path := "/" + segment + "/"
		if l.Parent != "" {
			parent, ok := byID[l.Parent]
			if !ok || parent.Organization != l.Organization || seen[l.Parent] {
				memo[id] = ""
				return "", false
			}
			seen[id] = true
			parentPath, ok := pathOf(l.Parent, seen)
			if !ok {
				memo[id] = ""
				return "", false
			}
			path = parentPath + segment + "/"
		}
		memo[id] = path
		return path, true
	}

	var broken []string
	written := 0
	for _, r := range rows {
		path, ok := pathOf(r.ID, map[string]bool{})
		if !ok {
			broken = append(broken, r.ID+" ("+r.Code+")")
			continue
		}
		if path == r.Path {
			continue
		}
		if _, err := app.DB().NewQuery("UPDATE {{locations}} SET [[path]] = {:path} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"path": path, "id": r.ID}).Execute(); err != nil {
			return err
		}
		written++
	}

	if len(broken) > 0 {
		log.Printf("⚠️ %d locations have a parent chain that loops or leaves their organization, and got no path: %s. "+
			"Make one of them a root or give it a correct parent, then save the rest once each, top down.", len(broken), strings.Join(broken, ", "))
	}
	log.Printf("✅ locations.path added; %d of %d locations filled", written, len(rows))
	return nil
}
