import { ref, watchEffect, type Ref } from 'vue'
import { fileUrl, type FileSource } from '@/utils/fileToken'

/**
 * Resolves a protected file's URL, minting a file token as needed.
 *
 * Every uploaded file on this platform is protected, so a URL without a token
 * 404s. That makes URL construction asynchronous, which is why this is a
 * composable and not a computed: `pb.files.getURL` is synchronous but the token
 * it needs is not.
 *
 * RESOLVED ONCE PER RECORD, NOT PER TOKEN. The returned ref changes when the
 * SOURCE changes, never because the token rotated. See utils/fileToken.ts for
 * why -- briefly, a reactive token re-fetches every image on screen every three
 * minutes, and one of this product's roles is an unattended wall display.
 *
 * USAGE -- pass a GETTER, not a value, so the source stays reactive:
 *
 *     const url = useFileUrl(() =>
 *       props.user?.avatar
 *         ? { record: props.user, filename: props.user.avatar, thumb: '100x100' }
 *         : null
 *     )
 *
 * Returns null while the token is in flight and whenever there is no file, so a
 * caller renders its fallback during both. That is deliberate: an avatar that
 * flashes a spinner is worse than one that appears a moment late, and the
 * fallback is a complete rendering rather than a placeholder for one.
 */
export function useFileUrl(source: () => FileSource | null | undefined): Ref<string | null> {
  const url = ref<string | null>(null)

  // Guards against an out-of-order resolution: if the source changes while a
  // token request is in the air, the older await can finish last and write a
  // URL for a record that is no longer being displayed. Cheap to prevent, and
  // it presents as one row in a list showing another row's image.
  let latest = 0

  watchEffect(() => {
    // Read the source SYNCHRONOUSLY, before any await. watchEffect only tracks
    // dependencies touched before the first suspension, so resolving this
    // inside the async body below would register no dependencies at all and the
    // effect would never re-run.
    const s = source()
    const seq = ++latest

    if (!s?.filename) {
      url.value = null
      return
    }

    void fileUrl(s).then((resolved) => {
      if (seq !== latest) return
      url.value = resolved
    })
  })

  return url
}
