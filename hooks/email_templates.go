package hooks

import (
	"bytes"
	"fmt"
	"html/template"
	"net/mail"
	"strings"
	texttemplate "text/template"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
)

// Operator-editable bodies for the emails the platform composes itself.
//
// PocketBase's own editable templates are a closed set of five on each auth
// collection, with no way to register a sixth, so the platform's own mails
// (the org invite) need a home of their own that an operator can edit without a
// rebuild.
//
// A collection rather than a file: it is editable in /_ beside the SMTP
// settings and travels with backups. All rules are nil, so it is
// superuser-only.
//
// The stored body is a fragment, and the shell below is added at send time,
// because the /_ rich-text editor strips <!DOCTYPE> and <html>.
//
// Every path falls back to the compiled-in default (missing or inactive row,
// blank field, a template that fails to parse or execute), so a broken edit
// cannot stop an invitation. The default is never removed.

// EmailTemplatesCollection is the collection holding those bodies.
const EmailTemplatesCollection = "email_templates"

// EmailTemplate is one message the platform can compose, in its compiled-in
// form. An active row in EmailTemplatesCollection with a matching Key overrides
// Subject and Body; this value is what the platform falls back to.
//
// Subject uses text/template, since escaping it would show &amp; to the reader.
// Body uses html/template, because it interpolates user-settable names that
// must not be able to close a tag.
type EmailTemplate struct {
	// Key matches the `key` column. Changing one orphans an operator's edits.
	Key string

	// Subject is a text/template source.
	Subject string

	// Body is an html/template source for the content only, with no document
	// shell.
	Body string

	// Description is written to the row when it is seeded, for the operator.
	// Never read back.
	Description string
}

// emailShellOpen and emailShellClose wrap a rendered body fragment into a
// plain, inline-styled, phone-width document.
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

// RegisterEmailTemplates creates a row for every compiled-in template that has
// none yet, so an operator in /_ finds an editable copy of each key.
//
// Seeded here rather than in a migration: the Go value stays the one source of
// the default, a template added later appears on existing installs, and an
// operator's edit is never overwritten.
//
// Nothing here can fail startup; the send path falls back regardless.
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
// default. A blank stored field falls through to the default, since clearing
// one is more likely an accident than a request.
func ResolveEmailTemplate(app core.App, def EmailTemplate) (subject, body string) {
	subject, body = def.Subject, def.Body

	rec, err := app.FindFirstRecordByFilter(
		EmailTemplatesCollection,
		"key = {:key} && active = true",
		dbx.Params{"key": def.Key},
	)
	if err != nil || rec == nil {
		// No collection, no row, or deactivated: the normal case, not worth a
		// log line.
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

	// The stored template is broken. Log at ERROR: the fallback mail still goes
	// out, so nothing else would reveal that the edit never took.
	app.Logger().Error(
		"stored email template failed to render, falling back to the built-in default",
		"collection", EmailTemplatesCollection,
		"key", def.Key,
		"error", err,
	)

	return renderEmailPair(def.Key, def.Subject, def.Body, data)
}

// renderEmailPair renders one subject/body pair, treating a failure in either as
// a failure of both, so an operator's subject never pairs with a built-in body.
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
// The sender comes from Settings().Meta. SenderName is preferred over AppName
// because PocketBase's own mails use it, so both send under one "from" name.
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
