import { describe, it, expect } from 'vitest'
import { isSystemSubject, sweepsSystemSubjects } from './systemSubjects'

describe('sweepsSystemSubjects', () => {
  it.each(['>', '*', '*.temp', '*.>'])('a leading wildcard can: %s', (p) => {
    expect(sweepsSystemSubjects(p)).toBe(true)
  })

  // An explicit prefix is the user asking for it, so it is never filtered.
  it.each(['sensor.>', 'sensor.*', '$KV.twin.>', '$JS.API.>', '_INBOX.>'])('a literal first token cannot: %s', (p) => {
    expect(sweepsSystemSubjects(p)).toBe(false)
  })
})

describe('isSystemSubject', () => {
  it.each([
    '$JS.API.CONSUMER.INFO.KV_twin.x',
    '$JS.ACK.stream.consumer.1.2.3.4.5',
    '$JS.EVENT.ADVISORY.API',
    '$KV.twin.thing.CAM-1.temp',
    '$SYS.REQ.ACCOUNT.PING.CONNZ',
    '$O.files.C.abc',
    '_INBOX.abc123.1',
  ])('system: %s', (s) => {
    expect(isSystemSubject(s)).toBe(true)
  })

  // `_INBOX` is a token, not a prefix: `_INBOXES.x` is somebody's own subject.
  it.each(['sensor.temp', 'hvac.AHU-1.state', '_INBOXES.x', 'inbox.1', 'price.$5'])('not system: %s', (s) => {
    expect(isSystemSubject(s)).toBe(false)
  })
})
