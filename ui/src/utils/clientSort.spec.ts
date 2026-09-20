import { describe, it, expect } from 'vitest'
import { sortBy } from './clientSort'

// Every failure mode here is SILENT. A sort that orders numbers as text still
// renders a full, plausible table; a sort that loses stability just makes rows
// twitch when you flip a column; a sort applied to the wrong array looks
// flawless on page one. None of them throws, and nothing in CI renders these
// two screens -- so the assertions are the only thing standing between a typo
// and a list that is confidently in the wrong order.

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
    // Under a naive String() compare with no numeric collation, '9' sorts after
    // '1024' and the biggest stream in the account hides mid-column.
    const rows = [{ messages: 9 }, { messages: 1024 }, { messages: 130 }]
    expect(sortBy(rows, 'messages').map(r => r.messages)).toEqual([9, 130, 1024])
    expect(sortBy(rows, '-messages').map(r => r.messages)).toEqual([1024, 130, 9])
  })

  it('takes the numeric path rather than leaning on numeric collation', () => {
    // The case above passes either way: `numeric: true` reads digit runs as
    // numbers, so it orders non-negative integers correctly by itself. These
    // two are where the paths disagree, which makes them the only assertions
    // that actually hold the `typeof === 'number'` branch in place. No column
    // in either JetStream list carries a decimal or a negative today; the point
    // is that the function is right about numbers, not about these numbers.
    expect(sortBy([{ v: 1.5 }, { v: 1.25 }], 'v').map(r => r.v)).toEqual([1.25, 1.5])
    expect(sortBy([{ v: 3 }, { v: -5 }], 'v').map(r => r.v)).toEqual([-5, 3])
  })

  it('treats zero as a value rather than as missing', () => {
    // `ttl: 0` renders as "None" and `consumers: 0` is a real count. Neither is
    // a blank to be swept to the bottom.
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
    // Stability is why the direction is folded into the comparator instead of
    // being a `.reverse()` at the end. A reverse would also flip the tied runs,
    // so retention -- a three-value enum, so almost all ties -- would reshuffle
    // its rows every time the column was clicked.
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
    // The caller passes a computed. Sorting it in place would mutate the value
    // a computed cached and make the order depend on how many times Vue chose
    // to re-evaluate it.
    const rows = [{ name: 'gamma' }, { name: 'alpha' }]
    const before = rows.map(r => r.name)
    sortBy(rows, 'name')
    expect(rows.map(r => r.name)).toEqual(before)
  })

  it('is only correct when the caller sorts before it slices', () => {
    // The invariant both JetStream views depend on, written as a test because
    // neither view can show it to a reviewer. What makes the mistake durable is
    // that it is INVISIBLE while the whole set fits on one page -- which is
    // every dev environment and most real accounts -- so it ships looking
    // perfect and only ever misbehaves for the people with the most data.
    const per = 20
    const rows = Array.from({ length: 45 }, (_, i) => ({ n: (i * 7) % 45 }))

    // Under one page: the slice is the whole set, so the two agree exactly.
    const small = rows.slice(0, 12)
    expect(sortBy(small, 'n').slice(0, per)).toEqual(sortBy(small.slice(0, per), 'n'))

    // Over one page: page two of the correct order is 20..39. Sorting the
    // already-sliced page returns whichever rows happened to land there,
    // in order -- a full, plausible, wrong page.
    const start = per
    const sortThenSlice = sortBy(rows, 'n').slice(start, start + per).map(r => r.n)
    const sliceThenSort = sortBy(rows.slice(start, start + per), 'n').map(r => r.n)

    expect(sortThenSlice).toEqual(Array.from({ length: per }, (_, i) => per + i))
    expect(sliceThenSort).not.toEqual(sortThenSlice)
  })

  it('leaves the list alone when the term names no field', () => {
    // A typo'd term should be inert, not scrambling. Every row reads
    // `undefined`, every comparison ties, and stability carries the input order
    // through.
    const rows = [{ name: 'gamma' }, { name: 'alpha' }, { name: 'beta' }]
    expect(sortBy(rows, 'mesages').map(r => r.name)).toEqual(['gamma', 'alpha', 'beta'])
  })
})
