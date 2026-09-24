// ui/src/utils/bufferView.ts
//
// Reading a widget buffer correctly. Two assumptions stopped holding once a
// message's timestamp could come from its payload or from JetStream, and once
// buffers started running full: position is not time, and length is not count.
import type { BufferedMessage } from '@/stores/widgetData'

/**
 * Messages that arrived after a paused snapshot was taken.
 *
 * Comparing lengths reads 0 as soon as the buffer is full -- a 200-message
 * buffer stays at 200 however much arrives -- so a paused console claimed
 * nothing was missed while it scrolled past thousands. The buffer keeps the
 * same message objects as it rotates, so find where the snapshot ended.
 *
 * If that message has already rotated out, everything now in the buffer
 * arrived since: the count is a floor, which `atLeast` says.
 */
export function missedSince(
  snapshot: readonly BufferedMessage[],
  live: readonly BufferedMessage[],
): { count: number; atLeast: boolean } {
  const last = snapshot[snapshot.length - 1]
  if (!last) return { count: live.length, atLeast: false }
  const i = live.lastIndexOf(last)
  if (i === -1) return { count: live.length, atLeast: true }
  return { count: live.length - 1 - i, atLeast: false }
}

/**
 * Newest first BY TIME. Walking the buffer backwards gives newest ARRIVAL
 * first, which differs when several subjects replay from JetStream or a
 * payload carries its own time. Stable, so equal timestamps keep arrival order.
 */
export function newestFirst<T extends BufferedMessage>(messages: readonly T[]): T[] {
  return [...messages].reverse().sort((a, b) => b.timestamp - a.timestamp)
}
