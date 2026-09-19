/**
 * Downscales an image in the browser before it is uploaded.
 *
 * WHY THIS EXISTS. `things.photo` and `locations.photo` are capped at 2 MiB,
 * and a phone camera produces 3-5 MB, so without this the cap would reject the
 * exact uploads the field is for. Raising the cap instead is the trade nobody
 * should take silently: files live in pb_data/storage with no object store
 * configured, things and locations are the collections with thousands of rows,
 * and every byte rides in every backup of a single-binary deployment.
 *
 * Nothing on screen needs more than 400x400 (the largest declared thumb), so a
 * 1600px long edge is already generous -- it leaves room for a future
 * full-size view without keeping the camera's original.
 *
 * SPLIT DELIBERATELY IN TWO. `targetDimensions` is pure arithmetic and is where
 * every decision lives; `downscaleImage` is the canvas work around it. The
 * split is not tidiness: vitest.config.ts runs in a node environment, jsdom
 * does not implement canvas.toBlob, so the canvas half cannot be tested here at
 * all. Keeping the judgement in a pure function means the untestable part is
 * only plumbing.
 */

/** The longest edge a stored photo keeps. */
export const MAX_DIMENSION = 1600

/** JPEG quality for re-encoded output. */
const QUALITY = 0.85

/**
 * The size to draw at, or null when the image is already small enough.
 *
 * Returning null rather than the original dimensions is the contract that keeps
 * an already-small file from being re-encoded: a 200x200 PNG run through a
 * canvas comes out a different, usually larger, JPEG for no reason.
 */
export function targetDimensions(
  width: number,
  height: number,
  max: number = MAX_DIMENSION,
): { width: number; height: number } | null {
  if (width <= 0 || height <= 0) return null
  const longest = Math.max(width, height)
  if (longest <= max) return null

  const scale = max / longest
  return {
    // round, not floor: a 1600x899.5 target floors to 899 and shifts the aspect
    // ratio by more than a rounding error on tall images.
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

/**
 * A downscaled copy of `file`, or the original when nothing would be gained.
 *
 * Never rejects. A browser that cannot decode the image, a canvas that refuses
 * to produce a blob, an animated format -- all of them fall back to uploading
 * what the user chose, and the server's own mimeTypes and maxSize are the
 * backstop. Failing the upload because an optimisation did not work would be
 * the wrong trade.
 */
export async function downscaleImage(file: File, max: number = MAX_DIMENSION): Promise<File> {
  // An SVG has no meaningful pixel dimensions to scale and a GIF would lose its
  // frames. Neither is accepted by the photo fields; this is here so the helper
  // is safe to point at another field later.
  if (!file.type.startsWith('image/') || file.type === 'image/svg+xml' || file.type === 'image/gif') {
    return file
  }

  let bitmap: ImageBitmap
  try {
    bitmap = await createImageBitmap(file)
  } catch {
    return file
  }

  try {
    const target = targetDimensions(bitmap.width, bitmap.height, max)
    if (!target) return file

    const canvas = document.createElement('canvas')
    canvas.width = target.width
    canvas.height = target.height
    const ctx = canvas.getContext('2d')
    if (!ctx) return file
    ctx.drawImage(bitmap, 0, 0, target.width, target.height)

    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob(resolve, 'image/jpeg', QUALITY)
    })
    if (!blob) return file

    // Re-encoding can lose: a large flat-colour PNG may beat JPEG at this size.
    // Keep whichever is actually smaller rather than assuming.
    if (blob.size >= file.size) return file

    // .jpg, because the bytes are now JPEG whatever the input was called. A
    // name that disagrees with its content is the kind of thing that surfaces
    // three systems downstream.
    const name = file.name.replace(/\.[^.]+$/, '') + '.jpg'
    return new File([blob], name, { type: 'image/jpeg', lastModified: Date.now() })
  } catch {
    return file
  } finally {
    bitmap.close()
  }
}
