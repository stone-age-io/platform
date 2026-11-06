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

/**
 * The inventory-events pair: the export `inventory-<organization id>` on the
 * operator account, the import `inventory-events` on each org's account.
 * Mirrors `inventoryExportName` and `inventoryImportName` in
 * hooks/inventory_exports.go -- KEEP THEM IN STEP.
 */
export const INVENTORY_IMPORT_NAME = 'inventory-events'
const INVENTORY_EXPORT_RE = /^inventory-[a-z0-9]{15}$/

/** Which mechanism provisions a record, so the banner can say how to remove it. */
export type ManagedSource = 'managed' | 'inventory'

export function managedSource(name: string | undefined | null): ManagedSource | null {
  if (!name) return null
  if (name === MANAGED_EXPORT_NAME || MANAGED_IMPORT_RE.test(name)) return 'managed'
  if (name === INVENTORY_IMPORT_NAME || INVENTORY_EXPORT_RE.test(name)) return 'inventory'
  return null
}

export function isManagedExport(name: string | undefined | null): boolean {
  return name === MANAGED_EXPORT_NAME || (!!name && INVENTORY_EXPORT_RE.test(name))
}

export function isManagedImport(name: string | undefined | null): boolean {
  return !!name && (MANAGED_IMPORT_RE.test(name) || name === INVENTORY_IMPORT_NAME)
}
