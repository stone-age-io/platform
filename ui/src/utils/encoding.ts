// Shared TextDecoder/TextEncoder instances, so callers do not create one per message.
export const Text_Decoder = new TextDecoder()
export const Text_Encoder = new TextEncoder()

export function decodeBytes(data: Uint8Array): string {
  return Text_Decoder.decode(data)
}

export function encodeString(str: string): Uint8Array {
  return Text_Encoder.encode(str)
}
