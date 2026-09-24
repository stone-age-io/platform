import { describe, it, expect } from 'vitest'
import { missedSince, newestFirst } from './bufferView'
import type { BufferedMessage } from '@/stores/widgetData'

const m = (timestamp: number, subject = 's'): BufferedMessage => ({ timestamp, value: timestamp, subject })

describe('missedSince', () => {
  it('counts what arrived after the snapshot', () => {
    const a = m(1), b = m(2), c = m(3), d = m(4)
    expect(missedSince([a, b], [a, b, c, d])).toEqual({ count: 2, atLeast: false })
  })

  // The bug: a full buffer keeps its length, so length arithmetic said 0.
  it('still counts once the buffer is full and rotating', () => {
    const a = m(1), b = m(2), c = m(3), d = m(4), e = m(5)
    const snapshot = [a, b, c]      // buffer of 3, full
    const live = [c, d, e]          // two more arrived, a and b rotated out
    expect(missedSince(snapshot, live)).toEqual({ count: 2, atLeast: false })
  })

  it('reports a floor when the whole snapshot has rotated out', () => {
    const snapshot = [m(1), m(2)]
    const live = [m(3), m(4)]
    expect(missedSince(snapshot, live)).toEqual({ count: 2, atLeast: true })
  })

  it('is zero when nothing arrived', () => {
    const a = m(1)
    expect(missedSince([a], [a])).toEqual({ count: 0, atLeast: false })
  })
})

describe('newestFirst', () => {
  it('orders by timestamp, not by arrival', () => {
    // subject A replayed first, then B: arrival order interleaves nothing
    const arrived = [m(10, 'A'), m(30, 'A'), m(20, 'B'), m(40, 'B')]
    expect(newestFirst(arrived).map(x => x.timestamp)).toEqual([40, 30, 20, 10])
  })

  it('keeps later arrivals first when timestamps tie', () => {
    const first = m(5, 'first'), second = m(5, 'second')
    expect(newestFirst([first, second]).map(x => x.subject)).toEqual(['second', 'first'])
  })

  it('does not mutate the buffer', () => {
    const buf = [m(1), m(2)]
    newestFirst(buf)
    expect(buf.map(x => x.timestamp)).toEqual([1, 2])
  })
})
