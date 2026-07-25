package hooks

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
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
// WHY THIS IS A ROUTE. `nats_accounts.updateRule` used to carry an owner/admin
// branch commented "can only change rotate_keys", implemented as a list of
// `:changed = false` assertions on the six limit fields plus `organization`.
// That is a deny-list, and it did not hold: an owner or admin could also write
// `jwt` (the signed account JWT), `revocations` (which user JWTs the account
// rejects), `public_key`, `signing_public_key`, `signing_keys`, `name`,
// `description` and `active`. Adding a field to the collection would have made it
// tenant-writable too, silently. Same failure this codebase has already been bitten
// by twice — a rule cannot express "this one field and nothing else".
//
// So the update rule is now operator-only, and the three operations a tenant
// legitimately needs live here. The switch below IS the allowlist: each action
// sets exactly one field, and an unrecognised action is rejected rather than
// ignored. There is no record id in the request — the account is derived from the
// caller's own active organization, so this cannot be aimed at another tenant.
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
// Deliberately NOT exposed: the account limits (`max_*`) and `jwt`. Limits are a
// platform-operator concern — they are what a tenant would raise to escape the
// resource envelope they were sold — and `jwt` is server-generated output, not
// input.
//
// Writes go through app.Save, so pb-audit records them as ordinary updates on
// nats_accounts with no extra work.
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
// organization, but only if the caller is an owner or admin there. The
// organization comes from the authenticated record, never from the request, so a
// caller cannot name another tenant's account.
//
// This mirrors the owner/admin allowlist used throughout schema.json. It is
// written as an explicit role check rather than reusing an API rule because the
// route bypasses the rules by design — the whole point is to permit a write the
// update rule now forbids.
func resolveOwnOrgNatsAccount(re *core.RequestEvent, opts NatsAccountRoutesOptions) (*core.Record, error) {
	if re.Auth == nil || re.Auth.Collection().Name != "users" {
		return nil, re.UnauthorizedError("user authentication required", nil)
	}

	orgID := re.Auth.GetString("current_organization")
	if orgID == "" {
		return nil, re.BadRequestError("no active organization selected", nil)
	}

	membership, err := re.App.FindFirstRecordByFilter(
		opts.MembershipCollection,
		"user = {:user} && organization = {:org} && (role = 'owner' || role = 'admin')",
		dbx.Params{"user": re.Auth.Id, "org": orgID},
	)
	if err != nil || membership == nil {
		// Same shape as an API-rule rejection: do not distinguish "not a member"
		// from "insufficient role".
		return nil, re.ForbiddenError("owner or admin of the active organization required", nil)
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
