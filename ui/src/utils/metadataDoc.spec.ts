import { describe, it, expect } from 'vitest'
import { describedKeys, undescribedEntries, mergeRows } from './metadataDoc'

// The invariant is the round trip: splitting a document between the schema
// form and the rows, then merging it back, must give the same document. A key
// that falls between the two halves is not an error anywhere — it is simply
// gone after the next save.

const schema = {
  type: 'object',
  properties: {
    asset_tag: { type: 'string', title: 'Asset tag' },
    warranty_months: { type: 'integer' },
  },
  required: ['asset_tag'],
}

const rowsOf = (entries: [string, any][]) => entries.map(([key, value]) => ({ key, value }))

describe('describedKeys', () => {
  it('names the schema properties', () => {
    expect([...describedKeys(schema)]).toEqual(['asset_tag', 'warranty_months'])
  })

  it('is empty when there is nothing to describe', () => {
    expect(describedKeys(null).size).toBe(0)
    expect(describedKeys({ type: 'array' }).size).toBe(0)
    expect(describedKeys({ type: 'object' }).size).toBe(0)
  })
})

describe('the metadata round trip', () => {
  const doc = {
    asset_tag: 'NW-0142',
    installer: 'Dana',
    warranty_months: 36,
    panel: { side: 'north' },
  }

  it('keeps keys the schema does not describe, with a schema', () => {
    const described = describedKeys(schema)
    const extras = undescribedEntries(doc, described)
    expect(extras.map(([k]) => k)).toEqual(['installer', 'panel'])
    expect(mergeRows(doc, described, rowsOf(extras))).toEqual(doc)
  })

  it('hands every key to the rows, without a schema', () => {
    const described = describedKeys(null)
    const entries = undescribedEntries(doc, described)
    expect(entries.map(([k]) => k)).toEqual(Object.keys(doc))
    expect(mergeRows(doc, described, rowsOf(entries))).toEqual(doc)
  })

  it('does not require a described key to be present', () => {
    const partial = { installer: 'Dana' }
    const described = describedKeys(schema)
    expect(mergeRows(partial, described, rowsOf(undescribedEntries(partial, described))))
      .toEqual(partial)
  })
})

describe('mergeRows', () => {
  const described = describedKeys(schema)

  it('adds a new row beside the described fields', () => {
    const next = mergeRows({ asset_tag: 'NW-1' }, described, [{ key: 'installer', value: 'Dana' }])
    expect(next).toEqual({ asset_tag: 'NW-1', installer: 'Dana' })
  })

  it('removes an extra key whose row was removed', () => {
    expect(mergeRows({ asset_tag: 'NW-1', installer: 'Dana' }, described, []))
      .toEqual({ asset_tag: 'NW-1' })
  })

  it('skips a row still waiting for its key', () => {
    expect(mergeRows({}, described, [{ key: '  ', value: 'x' }])).toBeNull()
  })

  it('lets the later of two rows with one key win', () => {
    const rows = [{ key: 'k', value: 1 }, { key: 'k', value: 2 }]
    expect(mergeRows({}, described, rows)).toEqual({ k: 2 })
  })

  it('returns null for an empty document', () => {
    expect(mergeRows({}, described, [])).toBeNull()
  })
})
