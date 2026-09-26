// ui/src/utils/systemSubjects.ts

/**
 * "Hide system subjects": a wildcard at the front of a subscription does not
 * sweep up NATS's own traffic. That one sentence is the whole feature.
 *
 * System means the first token starts with `$` (NATS reserves that prefix:
 * `$JS`, `$KV`, `$SYS`, `$O`) or is `_INBOX` (request/reply answers). A console
 * on `>` otherwise shows mostly the dashboard talking to itself -- every KV and
 * JetStream widget on the same connection makes `$JS.API` requests and gets
 * `_INBOX` replies, and NATS echoes a connection's own publishes back to it.
 *
 * Only a subscription whose FIRST token is a wildcard is filtered, so asking
 * for `$KV.twin.>` explicitly always delivers it, with no exception to reason
 * about. Twin changes on a quiet console are `['>', '$KV.twin.>']`. MQTT
 * behaves the same way: its wildcards never match `$` topics at the first
 * level.
 *
 * This saves no bandwidth. NATS has no negative subscription, so the browser
 * still receives everything `>` matches; the filter only keeps it out of the
 * buffer, where it would otherwise evict the messages you wanted.
 */

/** Can this subscription pattern match a system subject at all? */
export function sweepsSystemSubjects(pattern: string): boolean {
  const first = pattern.split('.', 1)[0]
  return first === '>' || first === '*'
}

/** Is this concrete subject NATS system traffic? */
export function isSystemSubject(subject: string): boolean {
  return subject.startsWith('$') || subject === '_INBOX' || subject.startsWith('_INBOX.')
}
