# migrations/CLAUDE.md - schema changes and migrations

Claude Code loads this file when it touches anything under `migrations/`. Every
`schema.json` change lands here as a new `schema_update_*.go`. If the change
touches an API rule, also read `AUTHORIZATION.md` and run
`./scripts/test-authz.sh`.

- **Schema changes need a new `migrations/schema_update_*.go`** — editing
  `schema.json` alone reaches fresh databases only. **A new non-null column with
  a live rule over it needs a backfill in the same migration**: PocketBase bools
  have no schema default, so existing rows land as `false`, and importing
  `authRule: "active = true"` without the `UPDATE` in
  `schema_update_device_active_flag.go` would lock every already-provisioned
  device out of the API on deploy.
- **`nats_roles` fields come from two places, and `schema.json` must list
  every one pb-nats uses.** pb-nats creates the collection before
  `schema.json` is imported, so a fresh database gets the library's fields
  regardless; a database older than one of the library's field additions did
  not, and PocketBase silently drops writes to it. That is how the role form's
  `allow_response` toggle stored nothing on early deployments. Declare a
  library's field with PocketBase's deterministic id (see the re-import bullet
  below); `migrations/role_response_fields_test.go` shows how to test it,
  since a fresh app passes whether or not the field is declared.
- **A re-import cannot REMOVE a field, and no migration here could until 2026-09.**
  `initial_schema.go` calls `ImportCollectionsByMarshaledJSON` with
  `deleteMissing=false`, and in that mode `core.ImportCollections` walks the live
  collection and re-adds every field the import did not carry by id
  (`core/collection_import.go`). So deleting a field from `schema.json` and
  re-importing logs its ✅ and changes nothing. Removal needs an explicit
  `Fields.RemoveById` + `Save`, keyed on the ID rather than the name for the same
  reason the importer is. `migrations/field_removal_test.go` pins both halves
  against the real importer, and `schema_update_drop_message_schemas.go` is the
  worked example — note that it deliberately does NOT re-import, since an import
  placed after the removals would resurrect them in the same run. Step order
  matters too: PocketBase refuses to delete a collection that still has relation
  references pointing at it, so a relation INTO a doomed collection goes first.
- **A removal migration cannot be tested by `migrate up`.** A fresh database
  imports the already-thinned `schema.json`, so the migration reaches nothing to
  remove and passes without touching anything. `drop_contract_layer_test.go`
  restores the old shape first, through the real importer, from
  `migrations/testdata/`. Any future field removal wants the same shape or its
  green tick means nothing.
- **A `schema.json` re-import silently keeps the LIVE field when a name matches
  and the id does not.** `core.ImportCollections` re-adds each existing field
  the import did not carry by id, and `FieldsList.add` then replaces the
  imported definition with the existing one — so the migration logs ✅ and
  changes nothing. Since every `schema_update_*.go` here is "re-import
  schema.json", this turns a whole class of migration into a no-op. It has bitten
  twice: `e72557f` (pb-nats recreated `account_id` as text) and the 25
  hand-written ids on `nats_account_exports`/`nats_account_imports`, whose
  definitions had never been applied anywhere. **Use PocketBase's deterministic
  id — `<type>` + crc32 of the field name — for anything a library also creates,
  and diff a real install before trusting that a re-import migration did what it
  says.** Only field definitions freeze this way; collection rules are applied
  normally, which is why those two collections' org-scoped rules are live while
  their field definitions were inert. Changing an id is safe: PocketBase reuses
  the existing field object rather than recreating the column.
