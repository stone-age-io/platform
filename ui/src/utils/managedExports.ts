// Identifies the platform-provisioned NATS export/import pairs so the console
// shows them read-only.
//
// Mirrors `managedExportName` and `hubImportName` in
// hooks/managed_org_exports.go -- KEEP THE TWO IN STEP (Go tests read this
// file). The names are Go constants, not configuration.
//
// Matching on the name is exact: `account_id` + `name` is the identity
// `ensureManagedExports` looks records up by, so a tenant-made record with this
// name would be adopted on the next org save anyway.
//
// `reconcile` reverts only: subject, type, description, organization,
// token_req and allowed_importers on the export; subject, type, description,
// organization, account and local_subject on the import. Other fields
// (`advertise`, `allow_trace`, service-response) would persist, but the console
// treats the whole record as read-only. The banner names the reverting fields.

/** The export's name, in the managed organization's own NATS account. */
export const MANAGED_EXPORT_NAME = 'helpdesk-events'

/**
 * The hub-side import is `<export>-<organization id>`. Matching the id shape,
 * not just the prefix, keeps a tenant's own `helpdesk-events-eu` editable.
 */
const MANAGED_IMPORT_RE = new RegExp(`^${MANAGED_EXPORT_NAME}-[a-z0-9]{15}$`)

export function isManagedExport(name: string | undefined | null): boolean {
  return name === MANAGED_EXPORT_NAME
}

export function isManagedImport(name: string | undefined | null): boolean {
  return !!name && MANAGED_IMPORT_RE.test(name)
}
