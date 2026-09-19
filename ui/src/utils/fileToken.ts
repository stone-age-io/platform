import { isTokenExpired } from 'pocketbase'
import { pb } from './pb'

/**
 * What to render: a record, the filename held by one of its file fields, and
 * optionally a thumb size.
 *
 * `record` is loose because every caller hands over something it already has --
 * an expanded relation, a form's in-flight record, a store's copy. pb.files
 * needs only the collection and the id.
 */
export interface FileSource {
  record: { id: string; collectionId?: string; collectionName?: string }
  filename: string
  /**
   * A thumb size, e.g. '100x100'. PocketBase serves '100x100' for any file
   * field whether or not the field declares it (apis/file.go,
   * `defaultThumbSizes`); any OTHER size must be in the field's own `thumbs`
   * list or the request silently falls through to the full original.
   */
  thumb?: string
}

/**
 * The cached file token, and the one place that asks the server for one.
 *
 * Every uploaded file on this platform is a PROTECTED field (schema.json,
 * migrations/schema_update_protected_files.go), so /api/files refuses to serve
 * one without a `token` query param that it resolves to an auth record and runs
 * against the owning collection's viewRule. The token is NOT the session token
 * -- it is a separate short-lived one minted by POST /api/files/token.
 *
 * THE MISTAKE THIS REPLACES: the two original avatar call sites passed
 * `pb.authStore.token`. That is the auth token, which /api/files does not
 * accept, and it worked only because the field was unprotected and the param
 * was therefore ignored. Copying that pattern onto a protected field gets a 404
 * with nothing in the console, so the wrong-looking-but-working version is
 * worth naming.
 *
 * WHY A CACHE. `users.fileToken.duration` is 180 seconds (schema.json), and a
 * list of thirty avatars would otherwise mint thirty tokens. One token covers
 * every file the caller can read, so this hands out the same one until it is
 * close to expiring, and collapses concurrent callers onto a single request.
 *
 * WHY IT IS NOT REACTIVE, which is the part worth getting right. If the token
 * were a ref, rotating it would change every URL derived from it and the
 * browser would re-fetch every image on the page -- every three minutes,
 * forever, on a screen nobody is touching. That is not hypothetical: the
 * `dashboard` role is an appliance login for an unattended display. So the
 * token is a plain module variable, URLs are resolved once per record (see
 * useFileUrl), and a rotation reaches only the images that mount after it.
 * Already-rendered images keep working because the server checked their token
 * when it served them, not since.
 */

// Refresh this far ahead of the stated expiry. Covers clock skew between the
// browser and the server plus the flight time of the request the token is
// about to be used in -- a token that expires while the image request is in the
// air fails, and the only symptom is a broken image.
const REFRESH_SKEW_SECONDS = 30

let cached = ''
let inFlight: Promise<string> | null = null

/**
 * A usable file token, or '' when there is no session to mint one against.
 *
 * Never throws: a file token is always in service of rendering an image, and a
 * caller that has to try/catch around an avatar will not. A failure returns ''
 * and the caller falls back to whatever it draws when there is no image.
 */
async function getFileToken(): Promise<string> {
  // isTokenExpired treats '' (and any unparseable token) as expired, so this
  // covers the cold-start case without a separate branch.
  if (cached && !isTokenExpired(cached, REFRESH_SKEW_SECONDS)) {
    return cached
  }

  // An unauthenticated caller cannot mint one, and asking produces a 401 in the
  // console on every logged-out page that happens to render an avatar.
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
 * Drops the cached token. Called on logout.
 *
 * The token outlives pb.authStore.clear() by up to its full duration, and it is
 * a bearer credential for every file the previous session could read. Clearing
 * it is not merely tidy: without this, the next user to log in on the same tab
 * would be handed the previous user's token for as long as it stayed valid, and
 * would silently see files their own account cannot.
 */
export function resetFileToken(): void {
  cached = ''
  inFlight = null
}

/**
 * A protected file's URL, or null when there is no file or no usable token.
 *
 * The imperative twin of the useFileUrl composable, for the call sites that are
 * already inside an async load function and just want a string -- a form
 * populating a preview, a map handing a URL to Leaflet. Prefer the composable
 * in a component that renders straight from a prop, since it re-resolves when
 * that prop changes; reach for this when the surrounding code already awaits.
 *
 * Returns null rather than throwing or returning a tokenless URL: a tokenless
 * URL for a protected file is a guaranteed 404, and a caller that renders it
 * gets a broken image instead of its own empty state.
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
