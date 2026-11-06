/** Nebula helpers shared by the CA, network and host views. */

import { pb } from '@/utils/pb'
import type { NebulaCA } from '@/types/pocketbase'

/**
 * Where a CA is in its rotation. DERIVED from the certificates, never stored
 * (as in pb-nebula): a status column could disagree with them after a crash.
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
 * Apply one rotation step to the active organization's CA.
 *
 * A route, not a record update: `nebula_ca.updateRule` is operator-only, since
 * a rule cannot allow one trigger field while forbidding the key material. The
 * route takes no id, so it cannot target another tenant.
 *
 * Show the server's error to the user: it names the out-of-order step or the
 * host blocking a finish (hooks/nebula_routes.go).
 */
export async function rotateCA(step: RotationStep): Promise<void> {
  await pb.send('/api/org/nebula-ca/rotate', { method: 'POST', body: { step } })
}

/**
 * Ids of active hosts whose certificate does not match their network's mask.
 * pb-nebula signed at /32 before v0.3.0, which gives a host a route covering
 * only itself: everything verifies and no packet crosses the mesh. The server
 * parses the certificates.
 *
 * A failure returns an empty set, not an exception: this drives an advisory
 * badge, and a list view must not fail to render because it is down.
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
 * The network a host's certificate should carry: its overlay IP at the
 * network's mask. Display only (the server does the real check); '' if the
 * CIDR is unreadable.
 */
export function expectedCertNetwork(overlayIp?: string, cidr?: string): string {
  if (!overlayIp || !cidr) return ''
  const bits = cidr.split('/')[1]
  if (!bits || !/^\d+$/.test(bits)) return ''
  return `${overlayIp}/${bits}`
}
