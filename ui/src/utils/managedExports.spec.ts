import { describe, expect, it } from 'vitest'
import { isManagedExport, isManagedImport } from './managedExports'

// The names come from hooks/managed_org_exports.go; the Go tests read
// managedExports.ts to keep the literals in step. What is pinned here is the
// matching: a platform-provisioned record is read-only, and a tenant's
// look-alike stays editable.
const ORG_ID = 'a1b2c3d4e5f6g7h'

describe('isManagedExport / isManagedImport', () => {
  it('splits the helpdesk pair by side', () => {
    expect(isManagedExport('helpdesk-events')).toBe(true)
    expect(isManagedImport(`helpdesk-events-${ORG_ID}`)).toBe(true)
    expect(isManagedImport('helpdesk-events')).toBe(false)
    expect(isManagedExport(`helpdesk-events-${ORG_ID}`)).toBe(false)
  })

  it('leaves look-alikes editable', () => {
    for (const name of ['helpdesk-events-eu', 'my-export', '', null, undefined]) {
      expect(isManagedExport(name)).toBe(false)
      expect(isManagedImport(name)).toBe(false)
    }
  })
})
