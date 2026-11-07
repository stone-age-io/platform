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

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
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
// Mailed links point at the console route /accept-invite, not here, so this
// path can change without breaking delivered mail.
const AcceptInvitePath = "/api/org/invites/accept"

// OrgInviteEmail is the compiled-in default for the organization invitation. An
// active `org_invite` row in email_templates overrides it. See
// hooks/email_templates.go for why it works that way. Body is a fragment; the
// document shell is added at send time.
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

// inviteEmailData is what the built-in template and any operator override are
// rendered against. Adding a field is safe; removing or renaming one silently
// blanks it in every customised template.
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
//   - Token and expiry are set on the MODEL hook, so seeds, migrations and tests
//     get them too; a blank token and zero expiry read as "expired".
//     `invited_by` stays on the request hook: it is the request's auth record.
//   - Resend reissues a fresh token and expiry before mailing, because the
//     console only offers Resend once an invitation has expired.
//   - Mail failures go to app.Logger(), so a failed invitation is never silent.
func RegisterInvites(app *pocketbase.PocketBase, opts InviteOptions) {
	app.OnRecordCreate(opts.InviteCollection).BindFunc(func(e *core.RecordEvent) error {
		// Only fill what is blank, so a superuser or a test can set either.
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

	app.OnRecordCreateRequest(opts.InviteCollection).BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Record.GetString("invited_by") == "" && e.Request != nil {
			if info, err := e.RequestInfo(); err == nil && info.Auth != nil {
				e.Record.Set("invited_by", info.Auth.Id)
			}
		}
		return e.Next()
	})

	// PocketBase defers *AfterCreateSuccess to commit, so the mailed token is
	// durable; the recipient of a rolled-back invitation could not tell.
	app.OnRecordAfterCreateSuccess(opts.InviteCollection).BindFunc(func(e *core.RecordEvent) error {
		if err := sendInviteEmail(e.App, e.Record, opts); err != nil {
			logInviteMailFailure(e.App, e.Record, err, "invite")
		}
		return e.Next()
	})

	// Resend: reissue, then mail. A post-commit second write, so the mail never
	// goes out ahead of its token; clearing the flag makes the re-entry a no-op.
	app.OnRecordAfterUpdateSuccess(opts.InviteCollection).BindFunc(func(e *core.RecordEvent) error {
		if !e.Record.GetBool("resend_invite") {
			return e.Next()
		}

		token, err := generateInviteToken()
		if err != nil {
			logInviteMailFailure(e.App, e.Record, err, "resend")
			return e.Next()
		}

		e.Record.Set("token", token)
		e.Record.Set("expires_at", inviteExpiry(opts))
		e.Record.Set("resend_invite", false)

		if err := e.App.Save(e.Record); err != nil {
			// Send nothing rather than a link that was never persisted.
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
// Unpadded, so no trailing "=" has to survive a query string.
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
		// Report it rather than send a mail with a blank organization name.
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

// inviteLink builds the console URL that redeems an invitation. It carries the
// email too, which AcceptInviteView uses to prefill the registration form.
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
// Requires auth: the invitee signs in or registers first, keeping account
// creation on the one path that validates it. So the caller's address is checked
// against the invitation, not trusted from it.
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

	// Case-insensitive: no real mail provider treats the local part as
	// case-sensitive.
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
		// Already in, usually a double-clicked link: clean up, no error.
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

		// Refetched, since e.Auth may be stale.
		user, err := txApp.FindRecordById(opts.UserCollection, authRecord.Id)
		if err != nil {
			return err
		}

		// Only when blank: accepting should not move you out of the org you are
		// working in. (RegisterOrgMembership sets it unconditionally for an owner.)
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
