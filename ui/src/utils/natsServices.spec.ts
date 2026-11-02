import { describe, it, expect } from 'vitest'
import { mergeServiceReplies } from './natsServices'

const info = (id: string, extra: object = {}) => ({
  type: 'io.nats.micro.v1.info_response',
  name: 'stone-agent', id, version: '0.4.0', description: 'Edge agent',
  metadata: { thing: 'S01' },
  endpoints: [{ name: 'reboot', subject: 'cmd.S01.reboot', queue_group: 'q' }],
  ...extra,
})

const stats = (id: string, endpoints: object[]) => ({
  type: 'io.nats.micro.v1.stats_response',
  name: 'stone-agent', id, version: '0.4.0', started: '2026-10-30T12:00:00Z', endpoints,
})

describe('mergeServiceReplies', () => {
  it('folds STATS into the matching INFO by instance id', () => {
    const [s] = mergeServiceReplies([info('a')], [stats('a', [
      { name: 'reboot', num_requests: 3, num_errors: 1, last_error: 'busy' },
      { name: 'ping', num_requests: 4, num_errors: 0, last_error: '' },
    ])])
    expect(s).toMatchObject({
      id: 'a', name: 'stone-agent', version: '0.4.0', metadata: { thing: 'S01' },
      endpoints: [{ name: 'reboot', subject: 'cmd.S01.reboot' }],
      started: '2026-10-30T12:00:00Z', requests: 7, errors: 1, lastError: 'busy',
    })
  })

  // No reply is not the same answer as no requests.
  it('leaves the numbers null when STATS did not arrive', () => {
    const [s] = mergeServiceReplies([info('a')], [stats('b', [])])
    expect(s.started).toBeNull()
    expect(s.requests).toBeNull()
    expect(s.errors).toBeNull()
  })

  it('drops a STATS reply with no INFO, and replies that are not micro services', () => {
    const out = mergeServiceReplies([info('a'), { hello: 'world' }, null], [stats('z', [])])
    expect(out.map(s => s.id)).toEqual(['a'])
  })

  it('tolerates the optional fields being absent', () => {
    const [s] = mergeServiceReplies([{ name: 'bare', id: 'x' }], [])
    expect(s).toMatchObject({ version: '', description: '', metadata: {}, endpoints: [] })
  })
})
