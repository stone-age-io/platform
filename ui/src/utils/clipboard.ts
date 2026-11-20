import { useToast } from '@/composables/useToast'

/**
 * Copy text and say whether it worked. `navigator.clipboard` is missing outside
 * a secure context (plain HTTP to anything but localhost), so a failed copy must
 * say so: the operator is about to paste a key or a config somewhere.
 */
export async function copyToClipboard(text: string, label: string): Promise<void> {
  const toast = useToast()
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch {
    toast.error('Failed to copy')
  }
}
