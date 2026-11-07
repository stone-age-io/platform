package migrations

import (
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_nats_role_limits stores -1 in every nats_roles limit that holds
// 0, and so re-mints every credential signed from those roles. No schema change.
// A role created without limits stored 0, which a user JWT reads as none;
// hooks/nats_limits.go now stores -1, and this corrects the older rows.
//
// Every 0 becomes -1, because no 0 did anything useful: Subs 0 is closed at
// CONNECT, Payload 0 on the first non-empty publish, and Data 0 is not enforced.
// -1 cannot exceed the tenant's envelope: nats-server applies the lower of the
// user and account limits, and account limits are operator-only.
//
// Saved through app.Save, not a plain UPDATE (contrast
// schema_update_device_active_flag.go): the defect is the credentials signed
// from the row. A role save makes pb-nats re-sign every active user holding it,
// after commit. Suspended users are re-signed when reactivated.
//
// A role that fails to save is logged and skipped, not fatal, so one role cannot
// keep every tenant down. nats_accounts is not touched: an account 0 may be an
// operator's deliberate choice, and hooks/nats_limits.go lets it be saved.
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
