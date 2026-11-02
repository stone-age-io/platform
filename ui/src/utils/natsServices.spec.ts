import { describe, it, expect } from 'vitest'
import { groupServices, mergeServiceReplies, metadataKeys, shortVersion } from './natsServices'

const info = (id: string, extra: object = {}) => ({
  type: 'io.nats.micro.v1.info_response',
  name: 'stone-agent', id, version: '0.4.0', description: 'Edge agent',
  metadata: { thing: 'S01' },
  endpoints: [
    { name: 'reboot', subject: 'cmd.S01.reboot', queue_group: 'q' },
    { name: 'ping', subject: 'cmd.S01.ping', queue_group: 'q' },
  ],
  ...extra,
})

const stats = (id: string, endpoints: object[]) => ({
  type: 'io.nats.micro.v1.stats_response',
  name: 'stone-agent', id, version: '0.4.0', started: '2026-10-30T12:00:00Z', endpoints,
})

describe('mergeServiceReplies', () => {
  it('folds STATS into the matching INFO by instance id, and each endpoint by name', () => {
    const [s] = mergeServiceReplies([info('a')], [stats('a', [
      { name: 'ping', num_requests: 4, num_errors: 0, last_error: '', average_processing_time: 2_500_000 },
      { name: 'reboot', num_requests: 3, num_errors: 1, last_error: 'busy', average_processing_time: 1_000_000 },
    ])])
    expect(s).toMatchObject({
      id: 'a', name: 'stone-agent', version: '0.4.0', metadata: { thing: 'S01' },
      started: '2026-10-30T12:00:00Z', requests: 7, errors: 1, lastError: 'busy',
    })
    expect(s.endpoints).toEqual([
      { name: 'reboot', subject: 'cmd.S01.reboot', queueGroup: 'q', requests: 3, errors: 1, lastError: 'busy', averageMs: 1 },
      { name: 'ping', subject: 'cmd.S01.ping', queueGroup: 'q', requests: 4, errors: 0, lastError: '', averageMs: 2.5 },
    ])
  })

  // No reply is not the same answer as no requests.
  it('leaves the numbers null when STATS did not arrive', () => {
    const [s] = mergeServiceReplies([info('a')], [stats('b', [])])
    expect(s.started).toBeNull()
    expect(s.requests).toBeNull()
    expect(s.errors).toBeNull()
    expect(s.endpoints[0].requests).toBeNull()
    expect(s.endpoints[0].averageMs).toBeNull()
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

describe('groupServices', () => {
  const instances = mergeServiceReplies(
    [info('a'), info('b', { version: '0.5.0' }), info('c'), info('d', { name: 'accessd', description: 'SoR', metadata: { host: 'x' } })],
    [stats('a', [{ name: 'reboot', num_requests: 2, num_errors: 1 }]), stats('b', [{ name: 'reboot', num_requests: 5, num_errors: 0 }])],
  )

  it('makes one group per name, versions ordered by how many instances run them', () => {
    const groups = groupServices(instances)
    expect(groups.map(g => [g.name, g.instances.length])).toEqual([['stone-agent', 3], ['accessd', 1]])
    expect(groups[0].versions).toEqual(['0.4.0', '0.5.0'])
  })

  it('sums only the instances whose STATS arrived, and is null when none did', () => {
    const [agents, accessd] = groupServices(instances)
    expect(agents.requests).toBe(7)
    expect(agents.errors).toBe(1)
    expect(accessd.requests).toBeNull()
  })

  it('lists metadata keys in first-seen order', () => {
    expect(metadataKeys(instances)).toEqual(['thing', 'host'])
  })
})

describe('shortVersion', () => {
  it('shortens a Go pseudo-version to its base and a short commit', () => {
    expect(shortVersion('0.1.1-0.20261013022711-733be73d0e58')).toBe('0.1.1 · 733be73')
    expect(shortVersion('v0.0.0-20261012023940-ff60f53bcb83')).toBe('0.0.0 · ff60f53')
    expect(shortVersion('0.0.0-20261012023940-ff60f53bcb83+dirty')).toBe('0.0.0 · ff60f53+dirty')
  })

  it('leaves every other version alone', () => {
    for (const v of ['0.4.0', 'v1.2.3', '0.0.0-dev', '1.0.0-rc.1', '']) expect(shortVersion(v)).toBe(v)
  })
})
