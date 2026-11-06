<script setup lang="ts">
/**
 * A user's uploaded avatar, or an initial circle when they have none.
 *
 * Use it only on screens gated to owner/admin or an operator. The `users`
 * viewRule is self / operator / org owner-or-admin, so for `member`, `viewer`
 * and `dashboard` an expand into `users` returns nothing and they would always
 * see the fallback. Not the activity feed either: `activity.actor` is a
 * plain-text snapshot, not a relation (hooks/activity.go).
 *
 * It fetches no record of its own; callers pass an already-expanded user.
 */
import { computed } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import type { User } from '@/types/pocketbase'

interface Props {
  /** Usually an `expand`ed relation; null when the row has no user (e.g. a system audit entry). */
  user?: Partial<User> | null
  /** Rendered diameter in px. */
  size?: number
  /** Fallback circle colour. Use 'primary' inside a tinted panel. */
  tone?: 'neutral' | 'primary'
  /**
   * Drawn only when the user has neither name nor email (absent or not yet
   * loaded). Pass a literal: a value derived from the name never shows,
   * because the name always wins first.
   */
  fallbackInitial?: string
}

const props = withDefaults(defineProps<Props>(), {
  user: null,
  size: 32,
  tone: 'neutral',
  fallbackInitial: '?',
})

const sizePx = computed(() => `${props.size}px`)

// 100x100 is PocketBase's built-in default thumb and `users.avatar` declares no
// others; any other size silently serves the full original.
const avatarUrl = useFileUrl(() => {
  const user = props.user
  if (!user?.avatar || !user.id) return null
  return { record: user as { id: string }, filename: user.avatar, thumb: '100x100' }
})

const initial = computed(() => {
  const name = props.user?.name || props.user?.email
  return name?.[0]?.toUpperCase() || props.fallbackInitial
})

const altText = computed(() => {
  const name = props.user?.name || props.user?.email
  return name ? `${name} avatar` : 'User avatar'
})
</script>

<template>
  <div
    v-if="avatarUrl"
    class="rounded-full overflow-hidden bg-base-200 flex-shrink-0"
    :style="{ width: sizePx, height: sizePx }"
  >
    <!-- object-cover: an avatar is a photo crop (OrgLogo uses contain, since a logo must not be cropped). -->
    <img :src="avatarUrl" :alt="altText" class="w-full h-full object-cover" />
  </div>

  <div
    v-else
    class="rounded-full flex items-center justify-center flex-shrink-0"
    :class="tone === 'primary' ? 'bg-primary text-primary-content' : 'bg-neutral text-neutral-content'"
    :style="{ width: sizePx, height: sizePx }"
    aria-hidden="true"
  >
    <span class="font-bold leading-none" :style="{ fontSize: `${Math.round(size * 0.4)}px` }">
      {{ initial }}
    </span>
  </div>
</template>
