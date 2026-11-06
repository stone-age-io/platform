import { isTokenExpired } from 'pocketbase'
import { pb } from './pb'

/**
 * A record (anything with an id and collection), the filename held by one of
 * its file fields, and optionally a thumb size.
 */
export interface FileSource {
  record: { id: string; collectionId?: string; collectionName?: string }
  filename: string
  /**
   * e.g. '100x100', which PocketBase serves for any file field. Any other size
   * must be in the field's `thumbs` list or the full original is served.
   */
  thumb?: string
}

/**
 * The cached file token, and the one place that mints one.
 *
 * Every file field is protected, so /api/files needs a `token` param checked
 * against the collection's viewRule. That is a file token from
 * POST /api/files/token, NOT `pb.authStore.token` (which gets a silent 404).
 *
 * Cached because `fileToken.duration` is 180s and one token covers every file;
 * concurrent callers share one request.
 *
 * Deliberately NOT reactive: a reactive token would change every derived URL on
 * rotation and re-fetch every image on the page every three minutes, and the
 * `dashboard` role runs unattended displays. URLs resolve once per record
 * (useFileUrl); a rotation reaches only images that mount after it.
 */

// Refresh this far ahead of expiry, for clock skew and the in-flight request.
const REFRESH_SKEW_SECONDS = 30

let cached = ''
let inFlight: Promise<string> | null = null

/**
 * A usable file token, or '' when there is no session or the mint failed.
 * Never throws: callers rendering an image will not try/catch.
 */
async function getFileToken(): Promise<string> {
  // isTokenExpired treats '' as expired, which covers the cold start.
  if (cached && !isTokenExpired(cached, REFRESH_SKEW_SECONDS)) {
    return cached
  }

  // Avoids a 401 on logged-out pages that render an avatar.
  if (!pb.authStore.isValid) {
    return ''
  }

  if (inFlight) return inFlight

  inFlight = pb.files
    .getToken()
    .then((token) => {
      cached = token
      return token
    })
    .catch((e) => {
      console.warn('Failed to mint a file token', e)
      return ''
    })
    .finally(() => {
      inFlight = null
    })

  return inFlight
}

/**
 * Drops the cached token. Called on logout: the token outlives
 * pb.authStore.clear() and is a bearer credential for every file the previous
 * session could read, so the next user on this tab must not inherit it.
 */
export function resetFileToken(): void {
  cached = ''
  inFlight = null
}

/**
 * A protected file's URL, or null when there is no file or no usable token
 * (a tokenless URL is a guaranteed 404).
 *
 * The imperative twin of useFileUrl, for code that already awaits. A component
 * rendering from a prop should use the composable.
 */
export async function fileUrl(source: FileSource | null | undefined): Promise<string | null> {
  if (!source?.filename || !source.record?.id) return null

  const token = await getFileToken()
  if (!token) return null

  return pb.files.getURL(source.record as any, source.filename, {
    thumb: source.thumb,
    token,
  })
}
