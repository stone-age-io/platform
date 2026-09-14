package hooks

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// InviteOptions names the collections the invitation flow touches.
type InviteOptions struct {
	MembershipCollection string
	InviteCollection     string
	UserCollection       string

	// ExpiryDays is how long a freshly issued or resent invitation stays valid.
	ExpiryDays int
}

// AcceptInvitePath is the endpoint the console posts an invitation token to.
//
// Renamed from /api/tenancy/accept-invite during the absorb: the old path named a
// library that no longer exists, and /api/org/... is the shape every other
// tenant-scoped route here already uses (see thing_routes.go). The invitation
// LINKS in already-delivered mail are unaffected -- they point at the console
// route /accept-invite, which posts here, not at the API directly.
const AcceptInvitePath = "/api/org/invites/accept"

// OrgInviteEmail is the compiled-in default for the organization invitation.
//
// This is a fallback, not the source of truth: an active `org_invite` row in
// email_templates overrides it, and that row is what an operator edits in /_.
// See hooks/email_templates.go for why it works that way.
//
// Body is a fragment -- no document shell, that is added at send time.
var OrgInviteEmail = EmailTemplate{
	Key:         "org_invite",
	Description: "Sent when someone is invited to an organization, and again when that invitation is resent.",
	Subject:     `You've been invited to join {{.OrgName}}`,
	Body: `<h2 style="margin:0 0 16px;font-size:20px;">You've been invited to join {{.OrgName}}</h2>
<p style="margin:0 0 16px;line-height:1.5;">{{.InviterName}} has invited you to join <strong>{{.OrgName}}</strong> on {{.AppName}} as a {{.Role}}.</p>
<p style="margin:0 0 28px;">
  <a href="{{.InviteLink}}" style="background:#18181b;color:#ffffff;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;font-weight:600;">Accept invitation</a>
</p>
<p style="margin:0 0 16px;line-height:1.5;color:#52525b;font-size:14px;">
  If the button does not work, paste this into your browser:<br>
  <span style="word-break:break-all;">{{.InviteLink}}</span>
</p>
<p style="margin:0;color:#71717a;font-size:12px;line-height:1.5;">
  This invitation expires on {{.ExpiresAt}}.<br>
  If you weren't expecting it, you can ignore this email.
</p>`,
}

// inviteEmailData is what both the built-in template above and any operator
// override are rendered against. Adding a field here is safe; removing or
// renaming one silently blanks it in every customised template, so treat these
// names as the contract they are.
type inviteEmailData struct {
	AppName     string
	OrgName     string
	Role        string
	InviterName string
	InviteLink  string
	ExpiresAt   string
}

// RegisterInvites owns the organization invitation lifecycle: issuing a token,
// mailing it, reissuing it, and redeeming it.
//
// ABSORBED FROM pb-tenancy. What changed, and why:
//
//   - Token and expiry moved from OnRecordCreateRequest to OnRecordCreate. The
//     request hook fires for REST calls only, so an invitation created by a seed,
//     a migration or a test got neither -- a row with a blank token and a zero
//     expiry, which the accept endpoint reads as "expired" and deletes. This is
//     the same trap CLAUDE.md already documents for pb-nats trigger fields: if it
//     has to happen on every path, it belongs on the MODEL hook. `invited_by`
//     stays on the request hook, because it is the request's auth record and has
//     no meaning without one.
//
//   - Resending now reissues. The original re-sent the mail and touched nothing
//     else, while the console only offers Resend once an invitation has EXPIRED
//     (InvitationsView.vue disables the button otherwise) -- so every Resend
//     mailed a link that was already dead, and redeeming it returned 410 and
//     deleted the invitation. Resend now mints a fresh token and a fresh expiry
//     before it mails anything.
//
//   - Failures are logged through app.Logger() rather than a fmt.Printf gated on
//     the library's LogToConsole flag, which config.yaml set to false. A failed
//     invitation email produced no output anywhere: the row was created, the
//     console showed it as pending, and nothing had gone wrong as far as anyone
//     could see.
//
//   - The mail body is html/template and comes from email_templates, not a Go
//     const parsed with text/template. InviterName is a user-settable display
//     name that was being interpolated into HTML unescaped.
func RegisterInvites(app *pocketbase.PocketBase, opts InviteOptions) {
	// Token and expiry, on every creation path.
	app.OnRecordCreate(opts.InviteCollection).BindFunc(func(e *core.RecordEvent) error {
		// Only fill what is blank. A superuser setting either field explicitly has
		// made a choice, and this is also what lets a test pin an expiry.
		if e.Record.GetString("token") == "" {
			token, err := generateInviteToken()
			if err != nil {
				return fmt.Errorf("generate invite token: %w", err)
			}
			e.Record.Set("token", token)
		}

		if e.Record.GetDateTime("expires_at").IsZero() {
			e.Record.Set("expires_at", inviteExpiry(opts))
		}

		return e.Next()
	})

	// Who sent it. Request-scoped by nature.
	app.OnRecordCreateRequest(opts.InviteCollection).BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Record.GetString("invited_by") == "" && e.Request != nil {
			if info, err := e.RequestInfo(); err == nil && info.Auth != nil {
				e.Record.Set("invited_by", info.Auth.Id)
			}
		}
		return e.Next()
	})

	// Mail it, once the row is committed. PocketBase defers *AfterCreateSuccess to
	// commit (core/db.go, txInfo.OnComplete), so the token in the link is durable
	// by the time it is sent -- which matters more here than anywhere, because the
	// recipient of a rolled-back invitation has no way to tell.
	app.OnRecordAfterCreateSuccess(opts.InviteCollection).BindFunc(func(e *core.RecordEvent) error {
		if err := sendInviteEmail(e.App, e.Record, opts); err != nil {
			logInviteMailFailure(e.App, e.Record, err, "invite")
		}
		return e.Next()
	})

	// Resend: reissue, then mail.
	app.OnRecordAfterUpdateSuccess(opts.InviteCollection).BindFunc(func(e *core.RecordEvent) error {
		if !e.Record.GetBool("resend_invite") {
			return e.Next()
		}

		// Post-commit, so this is a second write rather than part of the caller's.
		// That is the point: the mail must not go out ahead of the token it
		// carries, and a resent link is worthless if the row it names did not
		// land. Clearing the flag here also makes the re-entry a no-op, so there
		// is no loop.
		token, err := generateInviteToken()
		if err != nil {
			logInviteMailFailure(e.App, e.Record, err, "resend")
			return e.Next()
		}

		e.Record.Set("token", token)
		e.Record.Set("expires_at", inviteExpiry(opts))
		e.Record.Set("resend_invite", false)

		if err := e.App.Save(e.Record); err != nil {
			// Nothing is sent. The previous token is still whatever it was, which
			// is the safe outcome: mailing a link this process failed to persist
			// would be worse than not mailing one.
			logInviteMailFailure(e.App, e.Record, err, "resend")
			return e.Next()
		}

		if err := sendInviteEmail(e.App, e.Record, opts); err != nil {
			logInviteMailFailure(e.App, e.Record, err, "resend")
		}

		return e.Next()
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST(AcceptInvitePath, func(e *core.RequestEvent) error {
			return acceptInvite(e, opts)
		}).Bind(apis.RequireAuth())

		return se.Next()
	})
}

// generateInviteToken returns a cryptographically random, URL-safe token.
//
// RawURLEncoding rather than URLEncoding: 32 bytes pad to a trailing "=" under
// the padded alphabet, which then has to survive a query string intact. 43
// unpadded characters do not. Tokens issued before this change are unaffected --
// redemption is an exact-match lookup, not a parse.
func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func inviteExpiry(opts InviteOptions) time.Time {
	days := opts.ExpiryDays
	if days < 1 {
		days = 7
	}
	return time.Now().AddDate(0, 0, days)
}

func logInviteMailFailure(app core.App, invite *core.Record, err error, kind string) {
	app.Logger().Error("invitation email was not sent",
		"kind", kind,
		"invite", invite.Id,
		"email", invite.GetString("email"),
		"organization", invite.GetString("organization"),
		"error", err,
	)
}

// sendInviteEmail composes and sends the invitation for one invite record.
func sendInviteEmail(app core.App, invite *core.Record, opts InviteOptions) error {
	if errs := app.ExpandRecord(invite, []string{"organization", "invited_by"}, nil); len(errs) > 0 {
		// invited_by is optional and may legitimately be absent; organization is
		// required, and is checked below. A failure to expand either is worth
		// reporting rather than sending a mail with a blank organization name.
		return fmt.Errorf("expand invite relations: %v", errs)
	}

	org := invite.ExpandedOne("organization")
	if org == nil {
		return fmt.Errorf("invite %s has no organization", invite.Id)
	}

	meta := app.Settings().Meta
	appName := meta.AppName
	if appName == "" {
		appName = "the platform"
	}

	inviterName := "An administrator"
	if inviter := invite.ExpandedOne("invited_by"); inviter != nil {
		if name := strings.TrimSpace(inviter.GetString("name")); name != "" {
			inviterName = name
		} else if email := inviter.GetString("email"); email != "" {
			inviterName = email
		}
	}

	data := inviteEmailData{
		AppName:     appName,
		OrgName:     org.GetString("name"),
		Role:        invite.GetString("role"),
		InviterName: inviterName,
		InviteLink:  inviteLink(meta.AppURL, invite.GetString("token"), invite.GetString("email")),
		ExpiresAt:   invite.GetDateTime("expires_at").Time().Format("January 2, 2006"),
	}

	return SendTemplatedEmail(app, OrgInviteEmail, mail.Address{Address: invite.GetString("email")}, data)
}

// inviteLink builds the console URL that redeems an invitation.
//
// Carries the email as well as the token because the console's AcceptInviteView
// reads both: the token to redeem, the address to prefill the registration form
// an invitee without an account has to fill in first. The absorbed template sent
// only the token, so that field was always blank and every invitee retyped an
// address the system already knew.
func inviteLink(appURL, token, email string) string {
	q := url.Values{}
	q.Set("token", token)
	if email != "" {
		q.Set("email", email)
	}

	return strings.TrimRight(appURL, "/") + "/accept-invite?" + q.Encode()
}

// acceptInvite redeems a token for the authenticated caller.
//
// Requires auth, deliberately: the invitee signs in or registers first, and this
// endpoint only binds an existing account to an organization. That keeps account
// creation on the one path that already validates it, and it is why the caller's
// address is checked against the invitation below rather than trusted from it.
func acceptInvite(e *core.RequestEvent, opts InviteOptions) error {
	authRecord := e.Auth

	var body struct {
		Token string `json:"token" form:"token"`
	}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to read request data", err)
	}
	if body.Token == "" {
		return e.BadRequestError("Token is required", nil)
	}

	invite, err := e.App.FindFirstRecordByFilter(
		opts.InviteCollection,
		"token = {:token}",
		dbx.Params{"token": body.Token},
	)
	if err != nil {
		return e.NotFoundError("Invalid or expired invitation", err)
	}

	if time.Now().After(invite.GetDateTime("expires_at").Time()) {
		if err := e.App.Delete(invite); err != nil {
			e.App.Logger().Warn("could not delete expired invitation", "invite", invite.Id, "error", err)
		}
		return e.Error(http.StatusGone, "This invitation has expired", nil)
	}

	// Case-insensitive: the local part of an address is technically
	// case-sensitive, but no mail provider anyone uses treats it that way, and an
	// invitation addressed to Alice@example.com that its recipient cannot redeem
	// because they registered as alice@example.com is a support ticket, not a
	// security boundary.
	if !strings.EqualFold(authRecord.Email(), invite.GetString("email")) {
		return e.ForbiddenError("This invitation was issued to a different email address.", nil)
	}

	orgID := invite.GetString("organization")

	existing, _ := e.App.FindFirstRecordByFilter(
		opts.MembershipCollection,
		"user = {:user} && organization = {:org}",
		dbx.Params{"user": authRecord.Id, "org": orgID},
	)
	if existing != nil {
		// Already in. Redeeming is then just cleanup, and saying so plainly beats
		// an error for what is usually a double-clicked link.
		if err := e.App.Delete(invite); err != nil {
			e.App.Logger().Warn("could not delete redundant invitation", "invite", invite.Id, "error", err)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"message":       "You are already a member of this organization.",
			"alreadyMember": true,
		})
	}

	err = e.App.RunInTransaction(func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId(opts.MembershipCollection)
		if err != nil {
			return err
		}

		membership := core.NewRecord(collection)
		membership.Set("user", authRecord.Id)
		membership.Set("organization", orgID)
		membership.Set("role", invite.GetString("role"))
		if invitedBy := invite.GetString("invited_by"); invitedBy != "" {
			membership.Set("invited_by", invitedBy)
		}
		if err := txApp.Save(membership); err != nil {
			return err
		}

		// Refetched inside the transaction rather than reusing e.Auth, which was
		// loaded when the request was authenticated and may be stale.
		user, err := txApp.FindRecordById(opts.UserCollection, authRecord.Id)
		if err != nil {
			return err
		}

		// Only when blank. Accepting an invitation to someone else's organization
		// should not move you out of the one you are working in; the console's
		// switcher is how you go there. Contrast RegisterOrgMembership, which sets
		// this unconditionally for an org's own owner.
		if user.GetString("current_organization") == "" {
			user.Set("current_organization", orgID)
			if err := txApp.Save(user); err != nil {
				return err
			}
		}

		return txApp.Delete(invite)
	})
	if err != nil {
		return e.InternalServerError("Failed to accept invitation", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"message":      "Successfully joined organization.",
		"organization": orgID,
	})
}
