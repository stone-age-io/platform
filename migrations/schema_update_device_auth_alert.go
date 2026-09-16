package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_device_auth_alert turns off PocketBase's new-device login alert
// on the auth collections whose records are devices rather than people.
//
// `things` and `leaf_nodes` had it enabled -- the PocketBase default, never
// reconsidered when those collections were made auth collections. Their
// addresses are synthetic and undeliverable by construction: hooks/thing_routes.go
// mints `<code>@<org>.thing.local` precisely so the (organization, code) join key
// has somewhere to live, not so anyone can be written to. `nats_users` and
// `nebula_hosts` were already off and are only named here for completeness.
//
// WHY IT MATTERS, given that a failed send is swallowed. The send is BLOCKING.
// apis/record_helpers.go authAlert() fires the mail on a goroutine and then waits
// on it with a 15-second timer before the auth response is returned; the error is
// logged at Warn and authentication still succeeds, so the only symptom is
// latency. With no SMTP configured PocketBase falls back to mailer.Sendmail{},
// and the runtime image is alpine:3 with ca-certificates and tzdata -- there is
// no sendmail binary -- so the failure is immediate but the wait is real on every
// auth from an origin the record has not seen before. The fingerprint is
// MD5(ip + user-agent) and maxAuthOrigins is 5, so a device on a changing address
// keeps generating new origins and keeps paying.
//
// LEFT ENABLED ON `users` AND `_superusers`. Those records are people with real
// addresses, and "your account was used from somewhere new" is worth sending. It
// is also the one alert here that a human can act on.
//
// Import-only. The field is an option on the collection, not a column, so there
// is nothing to back-fill; no down, matching the other rule and option migrations
// in this directory.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping device auth alert update")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ New-device login alerts disabled on device auth collections (undeliverable synthetic addresses; the send blocks auth for up to 15s)")
		return nil
	}, nil)
}
