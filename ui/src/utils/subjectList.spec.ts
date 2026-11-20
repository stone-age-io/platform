import { describe, it, expect } from 'vitest'
import { formatForApi, formatForInput } from './subjectList'

describe('subject list round trip', () => {
  it('joins and splits back to the same subjects', () => {
    const subjects = ['telemetry.>', 'cmd.*.reply', '_INBOX.>']
    expect(formatForApi(formatForInput(subjects))).toEqual(subjects)
  })

  it('drops blanks and whitespace', () => {
    expect(formatForApi(' a.b ,, c.> , ')).toEqual(['a.b', 'c.>'])
  })

  it.each([undefined, null, '', []])('reads %s as no subjects', (val) => {
    expect(formatForApi(formatForInput(val))).toEqual([])
  })
})
