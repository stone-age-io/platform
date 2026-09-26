<script setup lang="ts">
/**
 * The OPERATOR's mark: the branding overlay's logo, or the default platform
 * SVG. Deliberately NOT the active organization's -- that is OrgLogo, and it
 * belongs in the org switcher.
 *
 * WHY THIS IS WORTH A COMMENT. This component used to resolve
 * `org logo > branding > default`. That priority arrived in adb8ca4 for
 * ui/src/views/badge/BadgeView.vue -- a printable employee identity card, where
 * showing the EMPLOYING organization's mark was exactly right. c19a78e deleted
 * that view, and the priority outlived its only justified consumer.
 *
 * What it left behind was a logo captioned with somebody else's name:
 * AppSidebar renders this immediately beside `brandingStore.appName`, so an
 * organization with a logo got its mark labelled with the operator's product
 * name. It also made the one fixed landmark in the chrome move -- this is the
 * router-link to `/` -- so it changed on every org switch, duplicating the
 * switcher sitting directly below it. And LoginView already disagreed, having
 * no org context to resolve, so the logo swapped the instant you authenticated.
 *
 * So a tenant's identity lives in the org switcher, where switching it is the
 * control. If per-organization white-labelling ever lands it has to move the
 * app NAME as well -- that is a feature, not a logo.
 */
import { computed } from 'vue'
import { useBrandingStore } from '@/stores/branding'

interface Props {
  size?: number | string
}

const props = withDefaults(defineProps<Props>(), {
  size: 48,
})

const brandingStore = useBrandingStore()

const sizePx = computed(() => (typeof props.size === 'number' ? `${props.size}px` : props.size))
</script>

<template>
  <!--
    Branding overlay logo: rendered as <img> so the file's own colors are
    preserved. The .brand-logo-img class is a hook for theme.css — operators can
    swap the source per theme via `content: url(...)`.
  -->
  <img
    v-if="brandingStore.logoUrl"
    :src="brandingStore.logoUrl"
    :alt="brandingStore.appName"
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
