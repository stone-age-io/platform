<script setup lang="ts">
import { computed } from 'vue'
import { useBrandingStore } from '@/stores/branding'
import { useAuthStore } from '@/stores/auth'
import { pb } from '@/utils/pb'

interface Props {
  size?: number | string
}

const props = withDefaults(defineProps<Props>(), {
  size: 48,
})

const brandingStore = useBrandingStore()
const authStore = useAuthStore()

const sizePx = computed(() => (typeof props.size === 'number' ? `${props.size}px` : props.size))

// Pick a thumb size that comfortably covers the rendered size at 2x DPR.
const thumb = computed(() => {
  const px = typeof props.size === 'number' ? props.size : parseInt(props.size as string, 10) || 48
  return px <= 50 ? '100x100' : '200x200'
})

// Resolution priority: org logo > global branding override > default platform SVG.
const orgLogoUrl = computed(() => {
  const org = authStore.currentOrg
  if (!org?.logo) return null
  return pb.files.getURL(org as any, org.logo, { thumb: thumb.value })
})

const resolvedLogoUrl = computed(() => orgLogoUrl.value || brandingStore.logoUrl)

const resolvedAlt = computed(() => {
  if (orgLogoUrl.value) return authStore.currentOrg?.name || brandingStore.appName
  return brandingStore.appName
})
</script>

<template>
  <!--
    Custom logo (org or global branding overlay): rendered as <img> so the file's
    own colors are preserved. The .brand-logo-img class is a hook for theme.css —
    operators can swap the source per theme via `content: url(...)`.
  -->
  <img
    v-if="resolvedLogoUrl"
    :src="resolvedLogoUrl"
    :alt="resolvedAlt"
    :style="{ width: sizePx, height: sizePx }"
    class="brand-logo-img object-contain"
  />

  <!--
    Default platform logo: inline SVG so currentColor (e.g. text-primary) tints
    it with the active DaisyUI primary color.
  -->
  <svg
    v-else
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 256 256"
    fill="currentColor"
    :style="{ width: sizePx, height: sizePx }"
  >
    <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z" />
  </svg>
</template>
