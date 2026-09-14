package hooks_test

import (
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"platform/hooks"
	"platform/internal/testutil"
)

// WHY THIS FILE EXISTS
//
// Everything under test here was pb-tenancy, and none of it was covered: the
// library shipped no tests and the platform could not write any against code it
// did not own. The absorb is the moment that stops being true, and the three bugs
// below are the ones that were live in production the day it happened.

func mustRecord(t *testing.T, app core.App, collection string) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatalf("find %s: %v", collection, err)
	}
	return core.NewRecord(col)
}

func mustSave(t *testing.T, app core.App, rec *core.Record) *core.Record {
	t.Helper()
	if err := app.Save(rec); err != nil {
		t.Fatalf("save %s: %v", rec.Collection().Name, err)
	}
	return rec
}

func newUser(t *testing.T, app core.App, email string) *core.Record {
	t.Helper()
	u := mustRecord(t, app, "users")
	u.Set("email", email)
	u.Set("password", "user-password-123")
	return mustSave(t, app, u)
}

func newOrg(t *testing.T, app core.App, name string, owner *core.Record) *core.Record {
	t.Helper()
	o := mustRecord(t, app, "organizations")
	o.Set("name", name)
	o.Set("active", true)
	o.Set("owner", owner.Id)
	return mustSave(t, app, o)
}

// An organization's owner needs the membership row, not just the relation: every
// write branch on the inventory collections resolves authority through
// memberships.role, so an owner without one cannot write in their own tenant.
func TestOrgCreateGivesTheOwnerAMembership(t *testing.T) {
	app := testutil.SetupApp(t)

	owner := newUser(t, app, "owner@example.test")
	org := newOrg(t, app, "Acme Industries", owner)

	membership, err := app.FindFirstRecordByFilter("memberships",
		"user = {:u} && organization = {:o}",
		map[string]any{"u": owner.Id, "o": org.Id})
	if err != nil || membership == nil {
		t.Fatalf("no membership created for the organization owner: %v", err)
	}
	if role := membership.GetString("role"); role != "owner" {
		t.Errorf("role = %q, want owner", role)
	}

	reloaded, err := app.FindRecordById("users", owner.Id)
	if err != nil {
		t.Fatalf("reload owner: %v", err)
	}
	if got := reloaded.GetString("current_organization"); got != org.Id {
		t.Errorf("current_organization = %q, want %q -- the inventory read rules scope on this field", got, org.Id)
	}
}

// THE REGRESSION THIS FILE IS MOST FOR.
//
// pb-tenancy's handler for this event was `return autoCreateOwnerMembership(...)`
// with no e.Next(), so it TERMINATED the chain -- and because the library bound it
// from inside its own OnBootstrap callback it was always last, which meant every
// handler bound after Bootstrap never ran. No handler, no error, no log.
// RegisterManagedOrgExports on an already-bootstrapped app provisioned nothing,
// and a Priority: -9999 bind was the only thing that fired.
//
// testutil.SetupApp returns a BOOTSTRAPPED app, so binding here is exactly the
// case that used to be silently inert.
func TestOrgCreateDoesNotTerminateTheHookChain(t *testing.T) {
	app := testutil.SetupApp(t)

	fired := false
	app.OnRecordAfterCreateSuccess("organizations").BindFunc(func(e *core.RecordEvent) error {
		fired = true
		return e.Next()
	})

	owner := newUser(t, app, "chain@example.test")
	newOrg(t, app, "Chain Test Co", owner)

	if !fired {
		t.Fatal("a handler bound after Bootstrap never ran: something in the organizations " +
			"AfterCreateSuccess chain is returning without calling e.Next(). That is the pb-tenancy " +
			"bug this absorb fixed -- see hooks/org_membership.go.")
	}
}

// Token and expiry moved from OnRecordCreateRequest to OnRecordCreate. The
// request hook fires for REST calls only, so an invitation created by a seed, a
// migration or a test got a blank token and a zero expiry -- which the accept
// endpoint reads as expired, and deletes. app.Save below IS that non-REST path.
func TestInviteCreateSetsTokenAndExpiryOffTheRequestPath(t *testing.T) {
	app := testutil.SetupApp(t)

	owner := newUser(t, app, "inviter@example.test")
	org := newOrg(t, app, "Invite Test Co", owner)

	invite := mustRecord(t, app, "invites")
	invite.Set("email", "invitee@example.test")
	invite.Set("organization", org.Id)
	invite.Set("role", "member")
	mustSave(t, app, invite)

	token := invite.GetString("token")
	if token == "" {
		t.Fatal("no token generated on a non-REST create -- the hook is bound to the request event again")
	}
	if len(token) != 43 {
		t.Errorf("token length = %d, want 43", len(token))
	}

	expires := invite.GetDateTime("expires_at").Time()
	if expires.IsZero() {
		t.Fatal("no expiry set on a non-REST create")
	}
	// main.go's default is 7 days; testutil passes the same.
	if d := time.Until(expires); d < 6*24*time.Hour || d > 8*24*time.Hour {
		t.Errorf("expires in %v, want roughly 7 days", d)
	}
}

// THE SECOND LIVE BUG. The console only offers Resend once an invitation has
// EXPIRED (InvitationsView.vue disables the button otherwise), and pb-tenancy's
// resend re-sent the mail without touching token or expiry -- so every Resend
// mailed a link that was already dead, and redeeming it returned 410 and deleted
// the invitation.
func TestResendReissuesTheInvitation(t *testing.T) {
	app := testutil.SetupApp(t)

	owner := newUser(t, app, "resender@example.test")
	org := newOrg(t, app, "Resend Test Co", owner)

	invite := mustRecord(t, app, "invites")
	invite.Set("email", "invitee@example.test")
	invite.Set("organization", org.Id)
	invite.Set("role", "member")
	mustSave(t, app, invite)

	originalToken := invite.GetString("token")

	// Put it in the state the console requires before it will offer Resend.
	invite.Set("expires_at", time.Now().AddDate(0, 0, -1))
	mustSave(t, app, invite)

	invite.Set("resend_invite", true)
	mustSave(t, app, invite)

	// Reread rather than trusting the in-memory copy: the reissue is a second,
	// post-commit write.
	reloaded, err := app.FindRecordById("invites", invite.Id)
	if err != nil {
		t.Fatalf("reload invite: %v", err)
	}

	if reloaded.GetString("token") == originalToken {
		t.Error("resend did not mint a new token -- the resent link is the one that already expired")
	}
	if expires := reloaded.GetDateTime("expires_at").Time(); !expires.After(time.Now()) {
		t.Errorf("resend left expires_at in the past (%v) -- the resent link is dead on arrival", expires)
	}
	if reloaded.GetBool("resend_invite") {
		t.Error("resend_invite was not cleared -- it would fire again on the next unrelated update")
	}
}

// A resend must not loop: clearing the flag inside the post-commit handler is
// what stops the second save from re-entering it forever.
func TestResendDoesNotRecurse(t *testing.T) {
	app := testutil.SetupApp(t)

	owner := newUser(t, app, "loop@example.test")
	org := newOrg(t, app, "Loop Test Co", owner)

	invite := mustRecord(t, app, "invites")
	invite.Set("email", "loop-invitee@example.test")
	invite.Set("organization", org.Id)
	invite.Set("role", "member")
	mustSave(t, app, invite)

	updates := 0
	app.OnRecordAfterUpdateSuccess("invites").BindFunc(func(e *core.RecordEvent) error {
		updates++
		if updates > 10 {
			t.Fatal("resend is re-entering itself")
		}
		return e.Next()
	})

	invite.Set("resend_invite", true)
	mustSave(t, app, invite)

	// The caller's write, plus the one reissue it triggers.
	if updates != 2 {
		t.Errorf("update handler ran %d times, want 2 (the caller's save and the single reissue)", updates)
	}
}

// The whole point of the email_templates collection: an operator edit in /_ has
// to win, and a broken one must not be able to stop the mail.
func TestEmailTemplateResolution(t *testing.T) {
	app := testutil.SetupApp(t)

	def := hooks.EmailTemplate{
		Key:     "probe_template",
		Subject: "built-in for {{.Name}}",
		Body:    "<p>built-in for {{.Name}}</p>",
	}
	data := map[string]any{"Name": "Acme"}

	subject, body, err := hooks.RenderEmail(app, def, data)
	if err != nil {
		t.Fatalf("render with no row: %v", err)
	}
	if subject != "built-in for Acme" {
		t.Errorf("subject = %q, want the built-in", subject)
	}
	if !strings.Contains(body, "built-in for Acme") {
		t.Errorf("body did not use the built-in: %q", body)
	}
	// The shell is added at send time, so the stored fragment need not carry one.
	if !strings.HasPrefix(body, "<!DOCTYPE html>") {
		t.Errorf("rendered body is not a document: %q", body)
	}

	row := mustRecord(t, app, "email_templates")
	row.Set("key", def.Key)
	row.Set("subject", "operator for {{.Name}}")
	row.Set("body", "<p>operator for {{.Name}}</p>")
	row.Set("active", true)
	mustSave(t, app, row)

	subject, body, err = hooks.RenderEmail(app, def, data)
	if err != nil {
		t.Fatalf("render with a row: %v", err)
	}
	if subject != "operator for Acme" {
		t.Errorf("subject = %q, want the operator's copy to win", subject)
	}
	if !strings.Contains(body, "operator for Acme") {
		t.Errorf("body = %q, want the operator's copy to win", body)
	}

	// Deactivated: back to the built-in, without the operator losing their draft.
	row.Set("active", false)
	mustSave(t, app, row)
	if subject, _, _ = hooks.RenderEmail(app, def, data); subject != "built-in for Acme" {
		t.Errorf("subject = %q, want the built-in once the row is inactive", subject)
	}

	// Broken: the operator who mistyped it is not the person waiting on the mail.
	row.Set("active", true)
	row.Set("body", "<p>{{.Name</p>")
	mustSave(t, app, row)

	subject, body, err = hooks.RenderEmail(app, def, data)
	if err != nil {
		t.Fatalf("a broken stored template must not fail the send: %v", err)
	}
	if !strings.Contains(body, "built-in for Acme") {
		t.Errorf("body = %q, want a fallback to the built-in", body)
	}
}

// The whole path, with the mailer intercepted instead of dialled: an invite is
// created, the hook expands its relations, renders the template and hands a
// message to the mail client.
//
// The case that matters is a BLANK invited_by. It is a legal state -- the field
// is optional, and a superuser-created invitation has no request auth to fill it
// from -- and sendInviteEmail expands it anyway, treating a non-empty error map
// as a failure. If expanding an empty relation reported an error, every such
// invitation would silently never be mailed, which is exactly the class of
// failure this absorb was cleaning up.
func TestInviteEmailIsComposedAndSent(t *testing.T) {
	app := testutil.SetupApp(t)

	var sent []string
	var subject, body, to string
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error {
		subject = e.Message.Subject
		body = e.Message.HTML
		if len(e.Message.To) > 0 {
			to = e.Message.To[0].Address
		}
		sent = append(sent, subject)
		// Do not call e.Next(): there is no SMTP server, and the point is what was
		// composed, not whether it left the building.
		return nil
	})

	owner := newUser(t, app, "sender@example.test")
	org := newOrg(t, app, "Mail Test Co", owner)

	invite := mustRecord(t, app, "invites")
	invite.Set("email", "invitee@example.test")
	invite.Set("organization", org.Id)
	invite.Set("role", "member")
	// invited_by deliberately left blank -- see the comment above.
	mustSave(t, app, invite)

	if len(sent) != 1 {
		t.Fatalf("mailer was handed %d messages, want 1. A blank invited_by most likely "+
			"made ExpandRecord report an error and sendInviteEmail bail before composing.", len(sent))
	}
	if to != "invitee@example.test" {
		t.Errorf("recipient = %q, want the invitee", to)
	}
	if subject != "You've been invited to join Mail Test Co" {
		t.Errorf("subject = %q", subject)
	}
	// The organization name resolved, so the expand actually ran.
	if !strings.Contains(body, "Mail Test Co") {
		t.Errorf("organization name missing from body: %q", body)
	}
	// No inviter, so the built-in stands in for one rather than leaving a gap.
	if !strings.Contains(body, "An administrator") {
		t.Errorf("expected the fallback inviter name in body: %q", body)
	}
	// The link has to carry the live token, not a placeholder.
	if !strings.Contains(body, invite.GetString("token")) {
		t.Errorf("invite token missing from the link in body: %q", body)
	}
}

// pb-tenancy parsed an HTML email with text/template, so nothing interpolated
// into it was escaped -- including InviterName, which comes from users.name and
// is set by whoever owns the account.
func TestInviteBodyEscapesUserControlledFields(t *testing.T) {
	app := testutil.SetupApp(t)

	_, body, err := hooks.RenderEmail(app, hooks.OrgInviteEmail, map[string]any{
		"AppName":     "Stone Age",
		"OrgName":     "Acme",
		"Role":        "member",
		"InviterName": `<script>alert(1)</script>`,
		"InviteLink":  "https://console.example.test/accept-invite?token=abc",
		"ExpiresAt":   "January 2, 2027",
	})
	if err != nil {
		t.Fatalf("render invite: %v", err)
	}

	if strings.Contains(body, "<script>") {
		t.Errorf("user-controlled display name reached the body unescaped -- "+
			"the template must be parsed with html/template, not text/template.\n%s", body)
	}
	if !strings.Contains(body, "alert(1)") {
		t.Error("the name was dropped entirely rather than escaped")
	}
}
