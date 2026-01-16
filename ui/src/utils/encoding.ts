/**
 * Global Encoding Utilities
 * Grug say: Don't make new machine every time you want to read.
 * Keep one machine, use it many times. Save memory.
 */

// Singleton instances
export const Text_Decoder = new TextDecoder()
export const Text_Encoder = new TextEncoder()

/**
 * Decode bytes to string
 */
export function decodeBytes(data: Uint8Array): string {
  return Text_Decoder.decode(data)
}

/**
 * Encode string to bytes
 */
export function encodeString(str: string): Uint8Array {
  return Text_Encoder.encode(str)
}
