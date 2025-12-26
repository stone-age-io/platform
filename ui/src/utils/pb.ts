import PocketBase from 'pocketbase'

/**
 * PocketBase client singleton
 * 
 * This is the single source of truth for all PocketBase API calls.
 * It's configured to use relative URLs (/) so it works both in dev (via Vite proxy)
 * and production (embedded in Go binary).
 */
export const pb = new PocketBase('/')

// Disable auto-cancellation for better UX
// This prevents requests from being cancelled when component unmounts
pb.autoCancellation(false)

// Enable persistent auth storage
// User stays logged in across page refreshes
pb.authStore.onChange(() => {
  // You could add global auth change handlers here if needed
})

export default pb
