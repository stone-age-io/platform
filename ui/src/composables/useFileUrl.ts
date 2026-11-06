import { ref, watchEffect, type Ref } from 'vue'
import { fileUrl, type FileSource } from '@/utils/fileToken'

/**
 * Resolves a protected file's URL, minting a file token as needed. Async
 * because the token is, hence a composable rather than a computed.
 *
 * Resolved once per record, not per token: the ref changes when the source
 * changes, never on token rotation (see utils/fileToken.ts).
 *
 * Pass a getter so the source stays reactive. Returns null while the token is
 * in flight and when there is no file, so callers render their fallback.
 */
export function useFileUrl(source: () => FileSource | null | undefined): Ref<string | null> {
  const url = ref<string | null>(null)

  // Drops an older resolution that finishes after a newer source.
  let latest = 0

  watchEffect(() => {
    // Read the source before any await: watchEffect tracks only what it
    // touches before the first suspension.
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
