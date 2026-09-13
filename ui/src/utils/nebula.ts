/**
 * Nebula overlay helpers shared by the CA, network and host views.
 *
 * Everything here is about the two things the console cannot work out on its
 * own. Rotation state is spread across three fields with no status column, and
 * whether a host certificate is still correct can only be answered by parsing
 * it — which needs Go, so the server answers instead.
 */

import { pb } from '@/utils/pb'
import type { NebulaCA } from '@/types/pocketbase'

/**
 * Where a CA is in its rotation.
 *
 * DERIVED, never stored — the same rule pb-nebula follows internally. A status
 * column could only ever disagree with the certificates that already say this,
 * and a rotation half-applied by a crash would leave the column lying while the
 * material told the truth.
 *
 *   idle      nothing in flight
 *   prepared  an incoming CA is trusted but not yet issuing (reversible)
 *   rotated   issuance has moved; the outgoing CA is still trusted
 */
export type CARotationState = 'idle' | 'prepared' | 'rotated'

export function caRotationState(ca: NebulaCA | null | undefined): CARotationState {
  if (!ca) return 'idle'
  if (ca.next_certificate) return 'prepared'
  if (ca.previous_certificate) return 'rotated'
  return 'idle'
}

export type RotationStep = 'prepare' | 'commit' | 'finish'

/**
 * Apply one rotation step to the caller's own organization's CA.
 *
 * Goes through a route rather than a record update because
 * `nebula_ca.updateRule` is operator-only: a PocketBase rule cannot permit one
 * trigger field while forbidding the certificate and key material on the same
 * record. The route takes no id — the CA is derived from the active
 * organization, so this cannot be aimed at another tenant.
 *
 * The server refuses an out-of-order step and names the host blocking a finish.
 * Let that message reach the operator; it is the whole reason the interlock is
 * usable. See hooks/nebula_routes.go.
 */
export async function rotateCA(step: RotationStep): Promise<void> {
  await pb.send('/api/org/nebula-ca/rotate', { method: 'POST', body: { step } })
}

/**
 * Ids of active hosts whose certificate no longer matches the network they
 * belong to.
 *
 * pb-nebula signed host certificates at /32 until v0.3.0. Nebula puts a
 * certificate's network straight onto the tun device and installs a link route
 * for it, so a /32 gives a host a route covering only itself: the certificate
 * verifies, the config renders, the handshake completes, and no packet ever
 * crosses the mesh. Nothing errors, which is why it went unnoticed for so long
 * — and why the console has to say so out loud.
 *
 * Answering this means parsing a Nebula certificate, so the server does it.
 *
 * A FAILURE RETURNS AN EMPTY SET, not an exception. This drives a warning badge
 * beside rows that are otherwise fine; a list view that refused to render
 * because an advisory endpoint was unreachable would be a worse outcome than a
 * missing badge. The caller shows what it has.
 */
export async function fetchStaleHostIds(): Promise<Set<string>> {
  try {
    const res = await pb.send<{ stale?: string[] }>('/api/org/nebula/cert-audit', {
      method: 'GET',
    })
    return new Set(res?.stale ?? [])
  } catch {
    return new Set()
  }
}

/**
 * The network a host's certificate SHOULD carry: its overlay IP at the
 * network's mask.
 *
 * Presentation only — the real comparison happens server-side against the
 * certificate itself. This exists so the badge can say what the operator is
 * fixing rather than only that something is wrong. Returns '' if the CIDR is
 * unreadable, and the caller drops the detail rather than showing a guess.
 */
export function expectedCertNetwork(overlayIp?: string, cidr?: string): string {
  if (!overlayIp || !cidr) return ''
  const bits = cidr.split('/')[1]
  if (!bits || !/^\d+$/.test(bits)) return ''
  return `${overlayIp}/${bits}`
}
