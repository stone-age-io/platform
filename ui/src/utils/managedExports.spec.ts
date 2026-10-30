import { describe, expect, it } from 'vitest'
import { isManagedExport, isManagedImport, managedSource } from './managedExports'

// The names come from hooks/managed_org_exports.go and hooks/inventory_exports.go;
// the Go tests read managedExports.ts to keep the literals in step. What is
// pinned here is the matching: a platform-provisioned record is read-only, and
// a tenant's look-alike stays editable.
const ORG_ID = 'a1b2c3d4e5f6g7h'

describe('managedSource', () => {
  it('recognises the helpdesk pair', () => {
    expect(managedSource('helpdesk-events')).toBe('managed')
    expect(managedSource(`helpdesk-events-${ORG_ID}`)).toBe('managed')
  })

  it('recognises the inventory pair', () => {
    expect(managedSource('inventory-events')).toBe('inventory')
    expect(managedSource(`inventory-${ORG_ID}`)).toBe('inventory')
  })

  it('leaves look-alikes editable', () => {
    for (const name of ['inventory-eu', 'inventory-events-eu', 'helpdesk-events-eu', 'my-export', '', null, undefined]) {
      expect(managedSource(name)).toBeNull()
      expect(isManagedExport(name)).toBe(false)
      expect(isManagedImport(name)).toBe(false)
    }
  })
})

describe('isManagedExport / isManagedImport', () => {
  it('splits each pair by side', () => {
    expect(isManagedExport(`inventory-${ORG_ID}`)).toBe(true)
    expect(isManagedImport('inventory-events')).toBe(true)
    expect(isManagedExport('helpdesk-events')).toBe(true)
    expect(isManagedImport(`helpdesk-events-${ORG_ID}`)).toBe(true)
  })
})
