package hooks

import (
	"database/sql"
	"errors"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// findFirst is FindFirstRecordByFilter with "not found" made plain: no row is
// (nil, nil), and every other error comes back to the caller.
//
// Use it instead of `x, _ := app.FindFirstRecordByFilter(...)`. That form reads
// a database error as "not found", and the code after it then creates a
// duplicate or skips a uniqueness check.
func findFirst(app core.App, collection any, filter string, params ...dbx.Params) (*core.Record, error) {
	rec, err := app.FindFirstRecordByFilter(collection, filter, params...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}
