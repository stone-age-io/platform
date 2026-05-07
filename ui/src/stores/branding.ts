import { defineStore } from 'pinia'
import { ref } from 'vue'

const DEFAULT_APP_NAME = 'Stone-Age.io'

interface BrandingManifest {
  appName?: string
  logo?: string
}

export const useBrandingStore = defineStore('branding', () => {
  const appName = ref<string>(DEFAULT_APP_NAME)
  const logoUrl = ref<string | null>(null)

  async function load() {
    try {
      const res = await fetch('/branding/branding.json', { cache: 'no-cache' })
      if (res.ok) {
        const manifest = (await res.json()) as BrandingManifest
        if (manifest.appName) appName.value = manifest.appName
        if (manifest.logo) logoUrl.value = `/branding/${manifest.logo}`
      }
    } catch {
      // No branding overlay configured — defaults stand.
    }
  }

  return { appName, logoUrl, load }
})
