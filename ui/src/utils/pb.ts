import PocketBase from 'pocketbase'

// Relative base URL: works in dev (Vite proxy) and when embedded in the binary.
export const pb = new PocketBase('/')

// Otherwise the SDK cancels a pending request when an identical one starts.
pb.autoCancellation(false)

export default pb
