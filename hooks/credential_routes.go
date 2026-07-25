package hooks

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// CredentialRoutesOptions names the collections involved, so a deployment that
// renamed them via config.yaml still works.
type CredentialRoutesOptions struct {
	NatsUserCollection   string
	MembershipCollection string
	ThingCollection      string
	LeafNodeCollection   string
}

// RegisterCredentialRoutes adds the one credential operation that PocketBase API
// rules cannot express: rotating your OWN NATS credentials.
//
// Reading credentials needs no route. The API rules in schema.json scope
// nats_users by ROW: an owner/admin sees every identity in their organization, and
// everyone else sees only the single identity belonging to them — the one their
// own browser, Thing, or leaf node authenticates with. Since a caller can only
// reach their own row, the credential inside it is not a leak.
//
// Rotation is different, because it is a WRITE. The update rule is owner/admin
// only, since publish_permissions on a nats_users record is copied verbatim into
// the JWT the server signs (pb-nats internal/jwt/generator.go) — a member who
// could write that field could self-grant publish ">" and be handed a signed JWT
// for it. Allowing self-rotation through the rule would mean permitting a write
// to exactly one field and no others, and a rule can only approximate that with
// `:isset = false` on every other writable field. That is a deny-list: it opens
// up silently the moment someone adds a field, which is the failure mode this
// codebase has already been bitten by. So rotation gets a route that sets the
// single field, and takes no record id — there is no other identity it could be
// aimed at.
//
// pb-nats watches `regenerate` on update, re-mints the user JWT and creds_file,
// then clears the flag (internal/sync/manager.go). Callers should re-read their
// own record afterwards to pick up the new credential.
//
// Deliberately NOT touched here: `revoke`. Setting `regenerate` on a revoked user
// re-enables it, so revocation has to stay an owner/admin action through the
// normal update rule. This route only ever writes `regenerate`.
//
// The write goes through app.Save, so pb-audit records it as an ordinary update
// on nats_users — rotations land in audit_logs with no extra work.
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

			// Exactly one field. This is the whole reason the route exists.
			rec.Set("regenerate", true)
			if err := re.App.Save(rec); err != nil {
				return re.InternalServerError("failed to rotate credentials", err)
			}

			return re.JSON(200, map[string]any{
				"rotated":   true,
				"nats_user": rec.Id,
			})
		}).Bind(apis.RequireAuth(
			// A leaf node is "a special thing"; both own exactly one identity.
			"users", opts.ThingCollection, opts.LeafNodeCollection,
		))

		return se.Next()
	})
}

// resolveOwnNatsUser derives the caller's single linked NATS identity from the
// authenticated record alone. Nothing is read from the request, so a caller
// cannot name someone else's identity.
//
// The three cases mirror the read branches in nats_users.listRule: a user reaches
// their identity through the membership for their active organization, while a
// Thing and a leaf node each carry the relation directly.
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

	case opts.ThingCollection, opts.LeafNodeCollection:
		natsUserID := re.Auth.GetString("nats_user")
		if natsUserID == "" {
			return "", re.NotFoundError("no NATS identity assigned yet", nil)
		}
		return natsUserID, nil

	default:
		// RequireAuth already restricts the collections; this keeps the handler
		// correct on its own terms if that binding is ever changed.
		return "", re.ForbiddenError("this identity type cannot rotate credentials", nil)
	}
}
