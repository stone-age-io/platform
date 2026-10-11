package migrations

import (
	"log"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_nats_role_limits stores -1 in every nats_roles limit that holds
// 0, and so re-mints every credential signed from those roles. No schema change.
//
// hooks/nats_limits.go now stores -1 for a limit left unset. Before it, a role
// created without its limits -- through the record API, `stone nats-role create`
// or `stone apply` -- stored 0, which a user JWT reads as NONE. This is the same
// correction for the rows written before the hook.
//
// THE DECISION: every 0 becomes -1. No 0 is kept as deliberate, because no 0 did
// anything a person could have wanted:
//
//   - Subscriptions 0: nats-server closes the connection at CONNECT, before the
//     first subscribe. A role at 0 has never had a user that could stay
//     connected, so it cannot have been a working publish-only role.
//   - Payload 0: the first non-empty publish closes the connection.
//   - Data 0: nats-server does not enforce it at all.
//
// -1 on a role cannot exceed the tenant's envelope: nats-server applies the lower
// of the user limit and the account limit, and the account limits are
// operator-only. What a user may publish or subscribe to is still the role's
// permission lists, which this does not touch.
//
// SAVED THROUGH app.Save, NOT A PLAIN UPDATE (contrast
// schema_update_device_active_flag.go). The row is not the defect; the
// credentials signed from it are. When a role is updated, pb-nats re-signs the
// JWT of every ACTIVE user holding it (setupRoleHooks, after the transaction
// commits). A plain UPDATE would leave each of those users holding Subs 0 until
// something unrelated re-signed it. Suspended users are skipped by pb-nats on
// purpose, and get a credential from the corrected role when reactivated.
//
// A role that cannot be saved is named in the log and left as it is, not made
// fatal: refusing to start the platform over one role would take every other
// tenant down with it. Save it by hand once the reason is fixed.
//
// nats_accounts is deliberately not touched. An account limit is the envelope
// the tenant was sold, an account's -1 is really unlimited, and a 0 there may be
// an operator's deliberate choice (a crude suspend from before
// organizations.active did anything). That is a decision for a person, and
// hooks/nats_limits.go is written so that a 0 already stored does not block
// later saves of the account.
func init() {
	m.Register(applyNatsRoleLimits, nil)
}

var natsRoleLimitColumns = []string{"max_subscriptions", "max_data", "max_payload"}

// applyNatsRoleLimits is the migration body. A fresh database has no roles at 0,
// so `migrate up` alone backfills nothing: nats_role_limits_test.go builds the
// broken rows and runs this again through the real migrations runner.
func applyNatsRoleLimits(app core.App) error {
	if _, err := app.FindCollectionByNameOrId("nats_roles"); err != nil {
		log.Printf("ℹ️ No nats_roles collection, so no NATS role limits to backfill: %v", err)
		return nil
	}

	where := make([]string, 0, len(natsRoleLimitColumns))
	for _, c := range natsRoleLimitColumns {
		where = append(where, "COALESCE([["+c+"]], 0) = 0")
	}
	roles, err := app.FindAllRecords("nats_roles", dbx.NewExp(strings.Join(where, " OR ")))
	if err != nil {
		return err
	}

	fixed := 0
	for _, role := range roles {
		var changed []string
		for _, c := range natsRoleLimitColumns {
			if role.GetInt(c) == 0 {
				role.Set(c, -1)
				changed = append(changed, c)
			}
		}
		if err := app.Save(role); err != nil {
			log.Printf("⚠️ NATS role %q (%s) still has %s at 0: %v",
				role.GetString("name"), role.Id, strings.Join(changed, ", "), err)
			continue
		}
		log.Printf("✅ NATS role %q (%s): %s 0 -> -1; its active users are re-signed",
			role.GetString("name"), role.Id, strings.Join(changed, ", "))
		fixed++
	}
	if fixed > 0 {
		log.Printf("✅ Backfilled -1 into the limits of %d NATS role(s)", fixed)
	}
	return nil
}
