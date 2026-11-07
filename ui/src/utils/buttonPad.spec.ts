import { describe, expect, it } from 'vitest'

import { inGrid, padCells, padPhoneHeight, padSize } from './buttonPad'
import { BUTTON_PAD_MAX, type ButtonPadCell } from '@/types/dashboard'

const cell = (row: number, col: number, label = `${row}:${col}`): ButtonPadCell =>
  ({ row, col, label, subject: 'a.b', payload: '{}' })

describe('padSize', () => {
  // Imported JSON is user-editable; a bad size must still draw a grid.
  it.each([
    [undefined, 1, 1],
    [{ rows: 3, cols: 4 }, 3, 4],
    [{ rows: 0, cols: -2 }, 1, 1],
    [{ rows: 99, cols: 2.7 }, BUTTON_PAD_MAX, 2],
    [{ rows: 'x' as any, cols: '3' as any }, 1, 3],
  ])('%j is %i x %i', (config, rows, cols) => {
    expect(padSize(config)).toEqual({ rows, cols })
  })
})

describe('padCells', () => {
  it('keeps only cells inside the grid', () => {
    const cells = padCells({ rows: 2, cols: 2, cells: [cell(0, 0), cell(1, 1), cell(2, 0), cell(0, 2), cell(-1, 0)] })
    expect(cells.map(c => c.label)).toEqual(['0:0', '1:1'])
  })

  // Two cells on one position would draw on top of each other.
  it('keeps the first of two cells on one position', () => {
    const cells = padCells({ rows: 1, cols: 1, cells: [cell(0, 0, 'first'), cell(0, 0, 'second')] })
    expect(cells.map(c => c.label)).toEqual(['first'])
  })

  it('copes with a missing or broken cell list', () => {
    expect(padCells(undefined)).toEqual([])
    expect(padCells({ rows: 2, cols: 2, cells: null as any })).toEqual([])
    expect(padCells({ rows: 2, cols: 2, cells: [null as any, cell(0, 0)] })).toHaveLength(1)
  })
})

describe('inGrid', () => {
  it('rejects fractional positions', () => {
    expect(inGrid({ row: 0.5, col: 0 }, 2, 2)).toBe(false)
  })
})

describe('padPhoneHeight', () => {
  // Every row must get at least a finger-sized key.
  it('grows with the row count and gives each row over 44px', () => {
    const one = padPhoneHeight({ rows: 1, cols: 3 })
    const four = padPhoneHeight({ rows: 4, cols: 3 })
    expect(four).toBeGreaterThan(one)
    expect((four - one) / 3).toBeGreaterThanOrEqual(44)
  })
})
