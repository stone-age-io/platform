// ui/src/utils/exportGrants.ts
//
// The importer list on a private NATS export (`allowed_importers`, pb-nats
// v0.3.3). For a `token_req` export, the platform signs the activation token for
// each listed account in this deployment that imports it, and nats-server refuses
// the import from any other account. Removing a key revokes it: pb-nats writes the
// key and the time into `revocations`, which the console only reads.

/**
 * A NATS account public key: `A` followed by 55 base32 characters. Checked here
 * because a mistyped key is not an error anywhere else -- the server stores it,
 * pb-nats never finds an account by it, and the intended importer simply gets
 * nothing.
 */
const ACCOUNT_KEY_RE = /^A[A-Z2-7]{55}$/

export function isAccountPublicKey(key: string): boolean {
  return ACCOUNT_KEY_RE.test(key)
}

/** Why `key` cannot be added to `list`, or null when it can. */
export function importerKeyError(key: string, list: readonly string[]): string | null {
  if (!key) return 'Enter an account public key'
  if (!isAccountPublicKey(key)) {
    return 'Not an account public key: it starts with A and is 56 characters'
  }
  if (list.includes(key)) return 'Already listed'
  return null
}

export interface RevokedImporter {
  key: string
  revokedAt: Date
}

/** The revocations map as a list, most recent first. */
export function revokedImporters(revocations: Record<string, number> | null | undefined): RevokedImporter[] {
  return Object.entries(revocations ?? {})
    .map(([key, unix]) => ({ key, revokedAt: new Date(unix * 1000) }))
    .sort((a, b) => b.revokedAt.getTime() - a.revokedAt.getTime())
}
