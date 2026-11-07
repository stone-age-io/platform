package hooks

import (
	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// CredentialRoutesOptions names the collections involved, so a deployment that
// renamed them via config.yaml still works.
type CredentialRoutesOptions struct {
	NatsUserCollection   string
	MembershipCollection string
	ThingCollection      string
}

// RegisterCredentialRoutes adds the one credential operation API rules cannot
// express: rotating your OWN NATS credentials. Reading needs no route, because
// nats_users is row-scoped and a non-admin reaches only their own identity.
//
// The nats_users update rule is owner/admin only: publish_permissions is copied
// into the signed JWT, so a member who could write it could grant themselves
// ">". Allowing self-rotation in the rule would need `:isset = false` on every
// other field, a deny-list that opens silently when a field is added. So this
// route sets the single field `regenerate`, and takes no record id.
//
// pb-nats re-mints the JWT and creds_file on `regenerate` and clears the flag;
// callers re-read their record afterwards. It never writes `revoke`, which must
// stay owner/admin. The write goes through app.Save, so pb-audit records it.
func RegisterCredentialRoutes(app *pocketbase.PocketBase, opts CredentialRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/me/nats-creds/rotate", func(re *core.RequestEvent) error {
			natsUserID, err := resolveOwnNatsUser(re, opts)
			if err != nil {
				return err
			}

			rec, err := re.App.FindRecordById(opts.NatsUserCollection, natsUserID)
			if err != nil {
				return re.NotFoundError("linked NATS identity not found", nil)
			}

			// A regenerate mints a JWT past the revocation cutoff, which NATS
			// accepts, so without this the rotate button is a self-service
			// un-suspend. Reactivating is owner/admin (or things.active).
			if !rec.GetBool("active") {
				return re.ForbiddenError("this NATS identity is suspended", nil)
			}

			// Exactly one field: the reason the route exists.
			rec.Set("regenerate", true)
			if err := re.App.Save(rec); err != nil {
				return re.InternalServerError("failed to rotate credentials", err)
			}

			return re.JSON(200, map[string]any{
				"rotated":   true,
				"nats_user": rec.Id,
			})
		}).Bind(apis.RequireAuth("users", opts.ThingCollection))

		return se.Next()
	})
}

// resolveOwnNatsUser derives the caller's single linked NATS identity from the
// authenticated record alone. Nothing is read from the request, so a caller
// cannot name someone else's identity.
//
// The two cases mirror the read branches in nats_users.listRule: a user goes
// through the membership for their active organization, a Thing carries the
// relation directly.
//
// It does not use requireMembership (hooks/org_context.go), deliberately:
//   - It applies NO role filter, and must not. Every console role, `viewer` and
//     `dashboard` included, may rotate its own credential; behind a helper that
//     takes a role list, a new role would silently lose that.
//   - Its denial is 404, not 403: with no role check there is nothing to hide,
//     and what is missing really is a resource.
//   - It returns the `nats_user`, not the organization, and half of it is the
//     Thing branch.
func resolveOwnNatsUser(re *core.RequestEvent, opts CredentialRoutesOptions) (string, error) {
	if re.Auth == nil {
		return "", re.UnauthorizedError("authentication required", nil)
	}

	switch re.Auth.Collection().Name {
	case "users":
		orgID := re.Auth.GetString("current_organization")
		if orgID == "" {
			return "", re.BadRequestError("no active organization selected", nil)
		}
		membership, err := re.App.FindFirstRecordByFilter(
			opts.MembershipCollection,
			"user = {:user} && organization = {:org}",
			dbx.Params{"user": re.Auth.Id, "org": orgID},
		)
		if err != nil {
			return "", re.NotFoundError("no membership in the active organization", nil)
		}
		natsUserID := membership.GetString("nats_user")
		if natsUserID == "" {
			return "", re.NotFoundError("no NATS identity linked to this organization context", nil)
		}
		return natsUserID, nil

	case opts.ThingCollection:
		natsUserID := re.Auth.GetString("nats_user")
		if natsUserID == "" {
			return "", re.NotFoundError("no NATS identity assigned yet", nil)
		}
		return natsUserID, nil

	default:
		// Unreachable behind RequireAuth; kept in case that binding changes.
		return "", re.ForbiddenError("this identity type cannot rotate credentials", nil)
	}
}
