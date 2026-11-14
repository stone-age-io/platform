package hooks

import (
	"database/sql"
	"errors"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// stubApp answers the few calls ensureManagedExports makes. A lookup in
// failOn returns a database error; every other lookup returns the record in
// records, or "no rows". Saves are recorded, not written.
type stubApp struct {
	core.App
	failOn  string
	records map[string]*core.Record
	saved   []string
}

var errDatabase = errors.New("database is locked")

func (a *stubApp) FindFirstRecordByFilter(collection any, _ string, _ ...dbx.Params) (*core.Record, error) {
	name, _ := collection.(string)
	if name == a.failOn {
		return nil, errDatabase
	}
	if rec := a.records[name]; rec != nil {
		return rec, nil
	}
	return nil, sql.ErrNoRows
}

func (a *stubApp) FindCollectionByNameOrId(name string) (*core.Collection, error) {
	col := core.NewBaseCollection(name)
	col.Id = name
	return col, nil
}

func (a *stubApp) Save(m core.Model) error {
	a.saved = append(a.saved, m.(*core.Record).Collection().Name)
	return nil
}

// A database error on the export or import lookup must not read as "absent":
// that would create a second export or import beside the existing one. Paired
// with the case where every lookup works, which must create both.
func TestEnsureManagedExportsRefusesOnALookupError(t *testing.T) {
	opts := ManagedOrgExportsOptions{
		OrgCollection:     "organizations",
		AccountCollection: "nats_accounts",
		ExportCollection:  "nats_exports",
		ImportCollection:  "nats_imports",
		ExportSubject:     "helpdesk.>",
	}
	newRec := func(collection, id string, fields map[string]any) *core.Record {
		rec := core.NewRecord(core.NewBaseCollection(collection))
		rec.Id = id
		for k, v := range fields {
			rec.Set(k, v)
		}
		return rec
	}
	org := newRec("organizations", "org1", map[string]any{"name": "Acme", "code": "acme", "managed": true})

	for _, c := range []struct {
		failOn  string
		wantErr bool
		want    []string
	}{
		{failOn: "", wantErr: false, want: []string{"nats_exports", "nats_imports"}},
		{failOn: "nats_exports", wantErr: true, want: nil},
		{failOn: "nats_imports", wantErr: true, want: []string{"nats_exports"}},
	} {
		name := "every lookup works"
		if c.failOn != "" {
			name = "lookup fails on " + c.failOn
		}
		t.Run(name, func(t *testing.T) {
			app := &stubApp{
				failOn: c.failOn,
				records: map[string]*core.Record{
					"nats_accounts": newRec("nats_accounts", "acct1", map[string]any{"public_key": "AKEY"}),
					"organizations": newRec("organizations", "hub", map[string]any{"is_operator_org": true}),
				},
			}

			err := ensureManagedExports(app, opts, org)
			if c.wantErr && !errors.Is(err, errDatabase) {
				t.Errorf("want the database error back, got %v", err)
			}
			if !c.wantErr && err != nil {
				t.Errorf("want no error, got %v", err)
			}
			if len(app.saved) != len(c.want) {
				t.Fatalf("saved %v, want %v", app.saved, c.want)
			}
			for i := range c.want {
				if app.saved[i] != c.want[i] {
					t.Errorf("saved %v, want %v", app.saved, c.want)
				}
			}
		})
	}
}
