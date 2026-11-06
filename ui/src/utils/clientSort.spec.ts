import { describe, it, expect } from 'vitest'
import { sortBy } from './clientSort'

// Every failure mode here is silent and nothing in CI renders these screens,
// so these assertions are the only guard.

describe('sortBy', () => {
  it('orders by a bare term ascending', () => {
    const rows = [{ name: 'gamma' }, { name: 'alpha' }, { name: 'beta' }]
    expect(sortBy(rows, 'name').map(r => r.name)).toEqual(['alpha', 'beta', 'gamma'])
  })

  it('orders by a `-` term descending', () => {
    const rows = [{ name: 'alpha' }, { name: 'gamma' }, { name: 'beta' }]
    expect(sortBy(rows, '-name').map(r => r.name)).toEqual(['gamma', 'beta', 'alpha'])
  })

  it('compares counts as numbers, not as plain text', () => {
    // A plain String() compare would put '9' after '1024'.
    const rows = [{ messages: 9 }, { messages: 1024 }, { messages: 130 }]
    expect(sortBy(rows, 'messages').map(r => r.messages)).toEqual([9, 130, 1024])
    expect(sortBy(rows, '-messages').map(r => r.messages)).toEqual([1024, 130, 9])
  })

  it('takes the numeric path rather than leaning on numeric collation', () => {
    // `numeric: true` collation alone gets non-negative integers right, so only
    // decimals and negatives hold the `typeof === 'number'` branch in place.
    expect(sortBy([{ v: 1.5 }, { v: 1.25 }], 'v').map(r => r.v)).toEqual([1.25, 1.5])
    expect(sortBy([{ v: 3 }, { v: -5 }], 'v').map(r => r.v)).toEqual([-5, 3])
  })

  it('treats zero as a value rather than as missing', () => {
    // `ttl: 0` and `consumers: 0` are real values, not blanks.
    const rows = [{ ttl: 3600 }, { ttl: 0 }, { ttl: 60 }]
    expect(sortBy(rows, 'ttl').map(r => r.ttl)).toEqual([0, 60, 3600])
  })

  it('sorts numbered names the way a person reads them', () => {
    const rows = [{ name: 'stream_10' }, { name: 'stream_2' }, { name: 'stream_1' }]
    expect(sortBy(rows, 'name').map(r => r.name)).toEqual(['stream_1', 'stream_2', 'stream_10'])
  })

  it('compares an array column as the text the cell renders', () => {
    const rows = [
      { subjects: ['orders.>'] },
      { subjects: ['audit.>', 'audit.admin.>'] },
    ]
    expect(sortBy(rows, 'subjects')[0].subjects[0]).toBe('audit.>')
  })

  it('keeps ties in their original order in BOTH directions', () => {
    // A `.reverse()` at the end would also flip tied runs, and enum columns
    // like retention are almost all ties.
    const rows = [
      { id: 'a', retention: 'limits' },
      { id: 'b', retention: 'limits' },
      { id: 'c', retention: 'workqueue' },
      { id: 'd', retention: 'limits' },
    ]
    expect(sortBy(rows, 'retention').map(r => r.id)).toEqual(['a', 'b', 'd', 'c'])
    expect(sortBy(rows, '-retention').map(r => r.id)).toEqual(['c', 'a', 'b', 'd'])
  })

  it('returns the original order for an empty term', () => {
    const rows = [{ name: 'gamma' }, { name: 'alpha' }]
    expect(sortBy(rows, '').map(r => r.name)).toEqual(['gamma', 'alpha'])
  })

  it('never mutates the array it was given', () => {
    // The caller passes a computed's cached value.
    const rows = [{ name: 'gamma' }, { name: 'alpha' }]
    const before = rows.map(r => r.name)
    sortBy(rows, 'name')
    expect(rows.map(r => r.name)).toEqual(before)
  })

  it('is only correct when the caller sorts before it slices', () => {
    // The invariant both JetStream views depend on. The mistake is invisible
    // while the whole set fits on one page.
    const per = 20
    const rows = Array.from({ length: 45 }, (_, i) => ({ n: (i * 7) % 45 }))

    // Under one page: the slice is the whole set, so the two agree exactly.
    const small = rows.slice(0, 12)
    expect(sortBy(small, 'n').slice(0, per)).toEqual(sortBy(small.slice(0, per), 'n'))

    // Over one page: page two of the correct order is 20..39. Sorting the
    // already-sliced page gives a plausible but wrong page.
    const start = per
    const sortThenSlice = sortBy(rows, 'n').slice(start, start + per).map(r => r.n)
    const sliceThenSort = sortBy(rows.slice(start, start + per), 'n').map(r => r.n)

    expect(sortThenSlice).toEqual(Array.from({ length: per }, (_, i) => per + i))
    expect(sliceThenSort).not.toEqual(sortThenSlice)
  })

  it('leaves the list alone when the term names no field', () => {
    // Every row reads `undefined`, every comparison ties, and stability keeps
    // the input order.
    const rows = [{ name: 'gamma' }, { name: 'alpha' }, { name: 'beta' }]
    expect(sortBy(rows, 'mesages').map(r => r.name)).toEqual(['gamma', 'alpha', 'beta'])
  })
})
