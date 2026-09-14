package hooks

import (
	"bytes"
	"fmt"
	"html/template"
	"net/mail"
	"strings"
	texttemplate "text/template"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

// Operator-editable bodies for the emails the platform composes itself.
//
// WHY THIS EXISTS AT ALL. PocketBase already has editable mail templates, and
// they are the right place for anything it sends on its own -- verification,
// password reset, confirm-email-change, OTP, the new-device auth alert. But that
// set is CLOSED: the five live on an auth collection as typed fields
// (core/collection_model_auth_options.go) and there is no mechanism, in the API
// or in /_, for registering a sixth kind. Any email the platform composes for
// ITSELF therefore has nowhere to live but Go.
//
// It lived in Go, in a library, as a const. Changing one word of the
// organization invite meant editing pb-tenancy, tagging it, `go get`, rebuild,
// redeploy -- for copy that an operator is exactly the right person to write and
// a developer is exactly the wrong one. That is the whole motivation.
//
// WHY A COLLECTION RATHER THAN A FILE. A file needs a volume and shell access to
// the container; a collection is editable in /_ by the superuser who is already
// there configuring SMTP, and it travels with the database into a backup. All
// five rules are nil, so it is superuser-only -- an email body is close enough to
// code that a tenant owner has no business editing one.
//
// WHY THE BODY IS A FRAGMENT. /_ renders an `editor` field as a rich-text box.
// Handed a full document it would fight the operator over <!DOCTYPE> and <html>,
// which TinyMCE strips. So the stored body is the CONTENT, the shell below is
// added at send time, and what the operator edits in /_ looks like what lands in
// the mailbox. Go template actions survive that round trip as plain text.
//
// WHY EVERY PATH FALLS BACK. A template that does not compile must not be able
// to stop an invitation: the operator who broke it is not the person waiting on
// the mail. A missing row, an inactive row, a blank field, and a template that
// fails to parse or execute all resolve to the compiled-in default. The default
// is the floor, not a seed -- it is never removed once a row exists.

// EmailTemplatesCollection is the collection holding those bodies.
const EmailTemplatesCollection = "email_templates"

// EmailTemplate is one message the platform can compose, in its compiled-in
// form. An active row in EmailTemplatesCollection with a matching Key overrides
// Subject and Body; this value is what the platform falls back to.
//
// Subject is rendered with text/template and Body with html/template. The split
// is deliberate: a subject line is not markup, and escaping it would put &amp;
// in front of the reader. A body IS markup, and the values interpolated into it
// include an organization name and a user-settable display name -- which is
// precisely the input that must not be able to close a tag. The library this
// replaced used text/template for both.
type EmailTemplate struct {
	// Key matches the `key` column and is the stable identifier. Changing one
	// orphans an operator's edits, so treat it like a collection name.
	Key string

	// Subject is a text/template source.
	Subject string

	// Body is an html/template source for the CONTENT of the mail, with no
	// document shell. See the comment above.
	Body string

	// Description is written to the row when it is seeded, to say what sends this
	// mail. It is for the operator reading the collection in /_ and is never read
	// back.
	Description string
}

// emailShellOpen and emailShellClose wrap a rendered body fragment into a
// document. Deliberately plain: table-free, inline-styled, and narrow enough for
// a phone. This is the part an operator does not have to think about.
const emailShellOpen = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:24px;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif;color:#18181b;">
<div style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:8px;padding:32px;">
`

const emailShellClose = `
</div>
</body>
</html>`

// RegisterEmailTemplates materialises a row for every compiled-in template that
// does not have one yet, so an operator opening EmailTemplatesCollection in /_
// finds the editable copy rather than an empty collection and no idea which keys
// exist.
//
// SEEDED HERE RATHER THAN IN A MIGRATION, for three reasons. The Go value stays
// the single source of the default, instead of being transcribed into a
// migration that then drifts from it. A template added in a later release shows
// up on an existing install without anyone writing another migration. And the
// create is conditional on absence, so an operator's edit is never overwritten
// -- which a re-running schema import could not promise.
//
// Nothing here can fail startup. A missing collection means the migration has
// not run yet, and the send path falls back to the compiled-in copy regardless.
func RegisterEmailTemplates(app *pocketbase.PocketBase, defs ...EmailTemplate) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		collection, err := se.App.FindCollectionByNameOrId(EmailTemplatesCollection)
		if err != nil {
			se.App.Logger().Warn("email template collection not found, built-in defaults will be used",
				"collection", EmailTemplatesCollection, "error", err)
			return se.Next()
		}

		for _, def := range defs {
			existing, _ := se.App.FindFirstRecordByFilter(
				EmailTemplatesCollection,
				"key = {:key}",
				dbx.Params{"key": def.Key},
			)
			if existing != nil {
				continue
			}

			rec := core.NewRecord(collection)
			rec.Set("key", def.Key)
			rec.Set("subject", def.Subject)
			rec.Set("body", def.Body)
			rec.Set("active", true)
			rec.Set("description", def.Description)

			if err := se.App.Save(rec); err != nil {
				se.App.Logger().Warn("could not seed email template",
					"key", def.Key, "error", err)
			}
		}

		return se.Next()
	})
}

// ResolveEmailTemplate returns the subject and body sources to render for def,
// preferring an active row in EmailTemplatesCollection over the compiled-in
// default.
//
// A blank stored field falls through to the default rather than sending an empty
// one -- clearing a subject in /_ is far more likely to be an accident than a
// request for a blank subject line.
func ResolveEmailTemplate(app core.App, def EmailTemplate) (subject, body string) {
	subject, body = def.Subject, def.Body

	rec, err := app.FindFirstRecordByFilter(
		EmailTemplatesCollection,
		"key = {:key} && active = true",
		dbx.Params{"key": def.Key},
	)
	if err != nil || rec == nil {
		// Missing collection, missing row, or deliberately deactivated. All three
		// mean "use the built-in", and none is worth a log line on every send --
		// an operator who has not customised a template is the normal case, not a
		// fault.
		return subject, body
	}

	if s := strings.TrimSpace(rec.GetString("subject")); s != "" {
		subject = s
	}
	if b := strings.TrimSpace(rec.GetString("body")); b != "" {
		body = b
	}

	return subject, body
}

// RenderEmail renders def against data, using the operator's stored copy when
// there is one and the compiled-in default when there is not. It returns the
// subject and the complete HTML document.
func RenderEmail(app core.App, def EmailTemplate, data any) (subject, html string, err error) {
	subjectSrc, bodySrc := ResolveEmailTemplate(app, def)

	subject, html, err = renderEmailPair(def.Key, subjectSrc, bodySrc, data)
	if err == nil {
		return subject, html, nil
	}

	// The stored template is broken. Say so at ERROR -- this one IS a fault, and
	// it is invisible from the console: the mail still goes out, correctly, which
	// is exactly why nobody would otherwise find out that their edit never took.
	app.Logger().Error(
		"stored email template failed to render, falling back to the built-in default",
		"collection", EmailTemplatesCollection,
		"key", def.Key,
		"error", err,
	)

	return renderEmailPair(def.Key, def.Subject, def.Body, data)
}

// renderEmailPair renders one subject/body pair, treating a failure in either as
// a failure of both. Mixing an operator's subject with a built-in body would
// produce a mail neither of them wrote.
func renderEmailPair(key, subjectSrc, bodySrc string, data any) (string, string, error) {
	st, err := texttemplate.New(key + ":subject").Parse(subjectSrc)
	if err != nil {
		return "", "", fmt.Errorf("parse subject: %w", err)
	}
	var subject bytes.Buffer
	if err := st.Execute(&subject, data); err != nil {
		return "", "", fmt.Errorf("execute subject: %w", err)
	}

	bt, err := template.New(key + ":body").Parse(bodySrc)
	if err != nil {
		return "", "", fmt.Errorf("parse body: %w", err)
	}
	var body bytes.Buffer
	if err := bt.Execute(&body, data); err != nil {
		return "", "", fmt.Errorf("execute body: %w", err)
	}

	return strings.TrimSpace(subject.String()), emailShellOpen + body.String() + emailShellClose, nil
}

// SendTemplatedEmail renders def and sends it to one recipient.
//
// The sender identity comes from Settings().Meta, so /_ stays the one place it is
// configured. SenderName is preferred over AppName because that is the field
// PocketBase's own mails use, and a deployment that set one and not the other
// should not get two different "from" names depending on which code sent the
// mail. The library this replaced used AppName, so those two disagreed.
func SendTemplatedEmail(app core.App, def EmailTemplate, to mail.Address, data any) error {
	subject, html, err := RenderEmail(app, def, data)
	if err != nil {
		return fmt.Errorf("render %q: %w", def.Key, err)
	}

	meta := app.Settings().Meta
	senderName := meta.SenderName
	if senderName == "" {
		senderName = meta.AppName
	}

	return app.NewMailClient().Send(&mailer.Message{
		From:    mail.Address{Name: senderName, Address: meta.SenderAddress},
		To:      []mail.Address{to},
		Subject: subject,
		HTML:    html,
	})
}
