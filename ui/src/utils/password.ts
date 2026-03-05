/**
 * Generate a random password of the specified length.
 * Uses a mix of lowercase, uppercase, digits, and special characters.
 */
export function generateRandomPassword(length = 16): string {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*'
  return Array.from({ length }, () => chars[Math.floor(Math.random() * chars.length)]).join('')
}
