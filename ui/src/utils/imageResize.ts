/**
 * Downscales an image in the browser before upload. The photo fields are capped
 * at 2 MiB and a phone produces 3-5 MB; raising the cap instead costs storage
 * and backup size (files live in pb_data/storage).
 *
 * Split so every decision is in the pure `targetDimensions`: jsdom has no
 * canvas.toBlob, so `downscaleImage` cannot be tested.
 */

/** The longest edge a stored photo keeps (the largest thumb is 400x400). */
export const MAX_DIMENSION = 1600

const QUALITY = 0.85

/**
 * The size to draw at, or null when the image is already small enough, so the
 * caller skips re-encoding (a small PNG through a canvas becomes a larger JPEG).
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
    // round, not floor, to keep the aspect ratio.
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

/**
 * A downscaled copy of `file`, or the original when nothing would be gained.
 * Never rejects: on any failure it returns the original, and the server's
 * mimeTypes and maxSize are the backstop.
 */
export async function downscaleImage(file: File, max: number = MAX_DIMENSION): Promise<File> {
  // SVG has no pixels to scale and a GIF would lose its frames.
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

    // A flat-colour PNG can beat JPEG; keep whichever is smaller.
    if (blob.size >= file.size) return file

    // The bytes are now JPEG, so the name must say so.
    const name = file.name.replace(/\.[^.]+$/, '') + '.jpg'
    return new File([blob], name, { type: 'image/jpeg', lastModified: Date.now() })
  } catch {
    return file
  } finally {
    bitmap.close()
  }
}
