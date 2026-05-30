/**
 * Generate a random password of the specified length.
 * Uses a mix of lowercase, uppercase, digits, and special characters.
 *
 * Backed by the Web Crypto CSPRNG (crypto.getRandomValues), not Math.random,
 * because these passwords become device/identity credentials. Bytes that would
 * introduce modulo bias are rejected so every character is uniformly likely.
 */
export function generateRandomPassword(length = 16): string {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*'
  const cutoff = Math.floor(256 / chars.length) * chars.length
  const out: string[] = []
  const buf = new Uint8Array(1)
  while (out.length < length) {
    crypto.getRandomValues(buf)
    if (buf[0] < cutoff) out.push(chars[buf[0] % chars.length])
  }
  return out.join('')
}
