<!-- ui/src/components/common/UserAvatar.vue -->
<script setup lang="ts">
/**
 * A person's mark: their uploaded avatar, or the initial circle when they have
 * none.
 *
 * WHY THIS EXISTS. `users.avatar` has been uploadable since the initial schema
 * and was rendered in exactly two places -- the account menu in AppSidebar and
 * the preview in UserSettingsView, both of them showing it only to its OWNER.
 * Every other surface that draws a person drew a hand-rolled initial circle
 * that never looked at the field: MembersView (row and card), MemberDetailView
 * (header and the invited-by chip), AuditLogView (row and card). So a user
 * could set an avatar and no one, including themselves on the Members page,
 * would ever see it. Six copies of the same markup, none of them wired up.
 *
 * WHERE IT MAY BE USED, which is a narrower list than it looks. `users` has a
 * viewRule of self / operator / org owner-or-admin, so `member`, `viewer` and
 * `dashboard` cannot read another user's record AT ALL -- an expand of a
 * relation into `users` returns nothing for them. This component is therefore
 * only correct on a screen already gated to owner/admin or to an operator.
 * Putting it somewhere every role can reach means some roles get a permanent
 * fallback circle for a user who does have an avatar, which is the
 * "gate on what the reader can READ" trap in CLAUDE.md.
 *
 * IN PARTICULAR, NOT THE ACTIVITY FEED. `activity.actor` is a plain-text
 * snapshot rather than a relation, deliberately (hooks/activity.go): a
 * non-cascade relation is blanked on user delete, and a log that changes when a
 * record changes is not a log. There is no user record to resolve there, and
 * every role reads that feed. Both halves say the same thing -- don't.
 *
 * NO RECORD FETCH OF ITS OWN, the same bargain OrgLogo makes. Every caller
 * already holds an expanded user record, and pb.files needs only its collection
 * and id, so rendering one per row costs nothing beyond the image. The one
 * request it does make is for a file token, cached across every caller on the
 * page (utils/fileToken.ts), so a members list mints one rather than one each.
 */
import { computed } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import type { User } from '@/types/pocketbase'

interface Props {
  /**
   * The user record. Partial because most callers hand over an `expand`ed
   * relation, and null because several call sites have a row whose user is
   * genuinely absent -- an audit entry written by the system, a membership
   * whose user failed to expand.
   */
  user?: Partial<User> | null
  /** Rendered diameter in px. */
  size?: number
  /**
   * The fallback circle's colour. 'neutral' is the default and is right
   * wherever the circle sits on the page background. 'primary' is for a mark
   * already inside a tinted panel -- the invited-by chip on MemberDetailView --
   * where a neutral circle reads as a second, unrelated element.
   */
  tone?: 'neutral' | 'primary'
  /**
   * The letter to draw when there is no name AND no email to take one from.
   * Callers mean different things by an absent user: AuditLogView attributes
   * those rows to the platform itself and passes 'S', so the circle has to be
   * able to say something other than "unknown".
   *
   * IT IS NARROWER THAN IT LOOKS, and two call sites got it wrong. `initial`
   * below prefers `name || email`, so this fires only when the record has
   * NEITHER -- an absent user, or one that has not hydrated yet. A signed-in
   * user always has an email (`AuthRecord.email` is required), so a value
   * DERIVED from a name is dead code here: whenever the derivation would yield
   * a letter, this component has already yielded the same one itself. Pass a
   * literal. AppSidebar computed one off `authStore.user.name` and
   * UserSettingsView computed one off the live form field, apparently to track
   * the name as you typed it; neither could ever be observed.
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

// 100x100 is PocketBase's own default thumb size (apis/file.go,
// `defaultThumbSizes`), served whether or not the field declares any -- and
// `users.avatar` declares none. So this is the ONLY thumb available here:
// asking for any other size falls through to the 5MB original with no error,
// which is the kind of failure this codebase keeps having to write down.
// Everything this component renders is chrome-sized, so 100x100 covers it even
// at 2x DPR.
//
// `users.avatar` is protected, so this is async -- it needs a file token, and
// the server checks it against the users viewRule. That rule is self /
// operator / org owner-or-admin, the same list in the WHERE IT MAY BE USED note
// above, so a screen that should not be drawing someone else's face now fails
// closed at the server rather than relying on the caller having read this
// comment.
const avatarUrl = useFileUrl(() => {
  const user = props.user
  if (!user?.avatar || !user.id) return null
  return { record: user as { id: string }, filename: user.avatar, thumb: '100x100' }
})

// The name is the source of the letter; `fallbackInitial` answers only the case
// where there is no name at all. A user with a name gets its first character
// even when the caller passed a fallback, because "S" on a row that names a
// person would be worse than no circle.
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
    <!-- object-cover, not contain: an avatar is a crop of a photo and letterboxing
         one inside a circle looks like a broken upload. OrgLogo makes the
         opposite call for the opposite reason -- a logo must not be cropped. -->
    <img :src="avatarUrl" :alt="altText" class="w-full h-full object-cover" />
  </div>

  <div
    v-else
    class="rounded-full flex items-center justify-center flex-shrink-0"
    :class="tone === 'primary' ? 'bg-primary text-primary-content' : 'bg-neutral text-neutral-content'"
    :style="{ width: sizePx, height: sizePx }"
    aria-hidden="true"
  >
    <!-- Scaled off the box rather than a fixed Tailwind step, so one component
         serves the 24px audit-log card and the 128px settings preview. -->
    <span class="font-bold leading-none" :style="{ fontSize: `${Math.round(size * 0.4)}px` }">
      {{ initial }}
    </span>
  </div>
</template>
