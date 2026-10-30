// ui/src/utils/managedExports.ts
//
// Identifies the NATS export/import pair that the platform provisions for a
// managed organization, so the console can present them as read-only instead of
// offering edits that quietly do nothing.
//
// Mirrors `managedExportName` and `hubImportName` in
// hooks/managed_org_exports.go -- KEEP THE TWO IN STEP. The Go side is the
// source of truth; it is a compile-time constant there, not configuration, so
// there is nothing to fetch at runtime. (`nats.managed_export_subject` is
// configurable; the record NAMES are not.)
//
// WHY MATCH ON THE NAME: because `account_id` + `name` is the exact identity
// `ensureManagedExports` looks the records up by. A record a tenant hand-made
// under this name would be adopted by the reconciler on the next org save, so
// the console marking it managed is not a false positive -- it is early notice.
//
// WHAT ACTUALLY REVERTS: `reconcile` rewrites only the fields in its desired
// list -- subject, type, description, organization on the export; those plus
// account and local_subject on the import. `token_req`, `advertise`,
// `allow_trace` and the service-response fields are create-only and an edit to
// them WOULD persist. The console still treats the whole record as read-only:
// splitting one form into a reconciled half and a durable half would need a
// per-field story in three views, and hand-tuning half of a platform-provisioned
// pipeline is not a thing to make easy. The banner names the fields that revert
// rather than claiming the record is frozen, because that is the true claim.

/** The export's name, in the managed organization's own NATS account. */
export const MANAGED_EXPORT_NAME = 'helpdesk-events'

/**
 * The hub-side import is named `<export>-<organization id>`. Matching the id
 * shape rather than just the prefix keeps a tenant's own `helpdesk-events-eu`
 * editable: only the server builds the 15-character form.
 */
const MANAGED_IMPORT_RE = new RegExp(`^${MANAGED_EXPORT_NAME}-[a-z0-9]{15}$`)

/**
 * The inventory-events pair (hooks/inventory_exports.go): the export sits on
 * the operator account as `inventory-<organization id>`, the import on each
 * organization's own account as `inventory-events`. Mirrors
 * `inventoryExportName` and `inventoryImportName` -- KEEP THEM IN STEP. Same
 * id-shaped match as the helpdesk import, so a hand-made `inventory-eu` stays
 * editable.
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
