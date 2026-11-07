package hooks

import (
	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// NatsAccountRoutesOptions names the collections involved, so a deployment that
// renamed them via config.yaml still works.
type NatsAccountRoutesOptions struct {
	NatsAccountCollection string
	MembershipCollection  string
}

// RegisterNatsAccountRoutes adds the signing-key operations an organization's
// owners and admins are meant to perform on their own NATS account.
//
// It is a route because a rule cannot express "this one field and nothing
// else". An owner/admin update branch would be a deny-list of `:changed =
// false` terms, which left `jwt`, `revocations` and the keys writable and would
// silently open every field added later. So `nats_accounts.updateRule` is
// operator-only, and the switch below IS the allowlist: each action sets
// exactly one field, and an unrecognised one is rejected. There is no record id
// in the request; the account comes from the caller's active organization.
//
// The three fields are pb-nats triggers, mutually exclusive and self-clearing
// (internal/sync/manager.go):
//
//   - rotate_keys        emergency replacement: purges every signing key and
//     generates one new one. Any user JWT signed by a purged
//     key stops validating, so this is the big hammer.
//   - add_signing_key    graceful rotation: appends a new signing key, leaving
//     existing ones valid.
//   - remove_signing_key removes one key by public key. pb-nats refuses to remove
//     the last remaining key.
//
// Deliberately NOT exposed: the limits (`max_*`), which would let a tenant
// escape the envelope it was sold, and `jwt`, which is server output. Writes
// go through app.Save, so pb-audit records them as ordinary updates.
func RegisterNatsAccountRoutes(app *pocketbase.PocketBase, opts NatsAccountRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/org/nats-account/keys", func(re *core.RequestEvent) error {
			var body struct {
				Action    string `json:"action"`
				PublicKey string `json:"public_key"`
			}
			if err := re.BindBody(&body); err != nil {
				return re.BadRequestError("invalid request body", err)
			}

			account, err := resolveOwnOrgNatsAccount(re, opts)
			if err != nil {
				return err
			}

			// The allowlist. One action, one field, nothing else reachable.
			switch body.Action {
			case "rotate":
				account.Set("rotate_keys", true)
			case "add_signing":
				account.Set("add_signing_key", true)
			case "remove_signing":
				if body.PublicKey == "" {
					return re.BadRequestError("public_key is required to remove a signing key", nil)
				}
				account.Set("remove_signing_key", body.PublicKey)
			default:
				return re.BadRequestError(
					`action must be one of "rotate", "add_signing", "remove_signing"`, nil)
			}

			if err := re.App.Save(account); err != nil {
				// pb-nats rejects removing the last signing key, among other things.
				return re.BadRequestError("key operation failed", err)
			}

			return re.JSON(200, map[string]any{
				"applied":      body.Action,
				"nats_account": account.Id,
			})
		}).Bind(apis.RequireAuth("users"))

		return se.Next()
	})
}

// resolveOwnOrgNatsAccount returns the NATS account of the caller's active
// organization, but only if the caller is an owner or admin there (the
// schema.json allowlist, restated because this route bypasses the rules). The
// organization comes from the authenticated record, never the request.
func resolveOwnOrgNatsAccount(re *core.RequestEvent, opts NatsAccountRoutesOptions) (*core.Record, error) {
	// The same gate as the Nebula routes.
	orgID, _, err := requireMembership(re, opts.MembershipCollection, rolesOrgManager)
	if err != nil {
		return nil, err
	}

	account, err := re.App.FindFirstRecordByFilter(
		opts.NatsAccountCollection,
		"organization = {:org}",
		dbx.Params{"org": orgID},
	)
	if err != nil || account == nil {
		return nil, re.NotFoundError("no NATS account for the active organization", nil)
	}
	return account, nil
}
