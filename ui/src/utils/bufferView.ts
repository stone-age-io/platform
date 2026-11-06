// Reading a widget buffer: position is not time (timestamps can come from the
// payload or JetStream), and length is not count (buffers run full).
import type { BufferedMessage } from '@/stores/widgetData'

/**
 * Messages that arrived after a paused snapshot was taken.
 *
 * Not a length comparison: a full buffer keeps its length. The buffer keeps the
 * same message objects as it rotates, so find where the snapshot ended. If that
 * message has rotated out, the count is a floor (`atLeast`).
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
