/**
 * "Hide system subjects": a wildcard at the front of a subscription does not
 * sweep up NATS's own traffic (first token starting with `$`, or `_INBOX`),
 * which is mostly the dashboard's own KV/JetStream requests and replies.
 *
 * Only a subscription whose FIRST token is a wildcard is filtered, so an
 * explicit `$KV.twin.>` is always delivered (as MQTT treats `$` topics).
 *
 * Saves no bandwidth (NATS has no negative subscription); it only keeps the
 * messages out of the buffer.
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
