package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_membership_timestamps adds `created` and `updated` autodate
// fields to memberships and invites.
//
// These two were the only collections in schema.json without them -- the other
// sixteen have carried the pair since the beginning -- and the console assumed
// otherwise. MembersView sorted `role,-created` and InvitationsView sorted
// `-created`, and PocketBase rejects an unknown sort term while PARSING the
// query (apis.recordsList -> searchProvider.ParseAndExec -> BadRequestError),
// before any API rule is evaluated. So both views answered 400 for every
// caller, superusers included, with the generic "Something went wrong while
// processing your request." that says nothing about which term was bad.
//
// The bug was latent from the start and only became total in d08b74d: before
// it, MembersView's loader sent `sort: role` and only the pager buttons sent
// `role,-created`, so page one worked and page two 400'd -- invisible on a
// member list under twenty rows. Consolidating both onto one options object
// (correctly, for filter reasons) moved the bad term onto first paint.
//
// Fixing it here rather than in the console is the choice that leaves the
// database able to answer "when did this person join" and "when was this invite
// sent" at all, which nothing could before.
//
// The field ids are PocketBase's deterministic defaults (autodate + crc32 of
// the name), which is what the other sixteen collections already use. That is
// load-bearing, not cosmetic: ImportCollections keeps the LIVE definition when
// a field's NAME matches but its id does not (core.ImportCollections ->
// FieldsList.add replaces the imported field with the existing one), so an
// invented id here would make this migration a silent no-op on any install that
// already has the pair. Same trap as e72557f's account_id.
//
// NO BACKFILL, and this one has a visible consequence, unlike
// schema_update_viewer_role.go. An autodate only fills on write, so rows that
// already exist keep the empty string and sort last under `-created`: on
// existing installs, pre-upgrade members appear at the bottom of their role
// group and pre-upgrade invites at the bottom of the list, until something
// updates them. That is deliberate. Nothing in the database records when those
// rows were created -- PocketBase ids are random, not time-ordered -- so any
// backfill would be an invented timestamp, and a made-up date is read as fact
// by everyone downstream. invites.expires_at would allow deriving one (it is
// created + tenancy.invite_expiry, and the resend hook does not refresh it),
// but only for invites, and a timestamp that is real on one collection and
// fabricated on the other is worse than one that is honestly absent on both.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping membership timestamps import")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Added created/updated to memberships + invites")
		return nil
	}, nil)
}
