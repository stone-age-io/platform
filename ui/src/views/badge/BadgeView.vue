<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNatsStore } from '@/stores/nats'
import { pb } from '@/utils/pb'
import { format } from 'date-fns'
import QRCode from 'qrcode'

const route = useRoute()
const authStore = useAuthStore()
const natsStore = useNatsStore()

const userName = computed(() => authStore.user?.name || 'User')
const userEmail = computed(() => authStore.user?.email || '')
const orgName = computed(() => authStore.currentOrg?.name || 'No Organization')
const roleName = computed(() => authStore.userRole)

// Badge users at /badge get badge dashboard, others at /my-badge get main dashboard
const isBadgeRoute = computed(() => route.path.startsWith('/badge'))
const dashboardPath = computed(() => isBadgeRoute.value ? '/badge/dashboard' : '/')

// Member since date
const memberSince = computed(() => {
  const created = authStore.currentMembership?.created
  if (!created) return null
  return format(new Date(created), 'MMM yyyy')
})

// Truncated NKey public key preview
const truncatedNKey = computed(() => {
  const pk = authStore.currentNatsUser?.public_key
  if (!pk || pk.length < 12) return pk || null
  return pk.slice(0, 6) + '...' + pk.slice(-4)
})

// Whether the user has a NATS identity
const hasNatsIdentity = computed(() => !!authStore.currentNatsUser)

function getAvatarUrl() {
  if (!authStore.user?.avatar) return null
  return pb.files.getURL(authStore.user, (authStore.user as any).avatar, {
    thumb: '200x200',
    token: pb.authStore.token
  })
}

// NATS status indicator
const natsStatusDotClass = computed(() => {
  switch (natsStore.status) {
    case 'connected': return 'live--connected'
    case 'connecting':
    case 'reconnecting': return 'live--warning'
    default: return 'live--error'
  }
})

const natsStatusLabel = computed(() => {
  switch (natsStore.status) {
    case 'connected': return 'LIVE'
    case 'connecting':
    case 'reconnecting': return 'SYNC'
    default: return 'OFF'
  }
})

// QR code generation — encode NKey public key only (~56 chars, clean scannable QR)
const qrDataUrl = ref<string | null>(null)

watch(
  () => authStore.currentNatsUser?.public_key,
  async (publicKey) => {
    if (!publicKey) {
      qrDataUrl.value = null
      return
    }
    try {
      qrDataUrl.value = await QRCode.toDataURL(publicKey, {
        width: 160,
        margin: 1,
        color: { dark: '#000000', light: '#ffffff' },
        errorCorrectionLevel: 'M'
      })
    } catch {
      qrDataUrl.value = null
    }
  },
  { immediate: true }
)

// Live clock
const now = ref(new Date())
let clockInterval: ReturnType<typeof setInterval> | null = null

const clockTime = computed(() => format(now.value, 'h:mm:ss a'))
const clockDate = computed(() => format(now.value, 'EEE, MMM d'))

onMounted(() => {
  clockInterval = setInterval(() => { now.value = new Date() }, 1000)
})

onUnmounted(() => {
  if (clockInterval) clearInterval(clockInterval)
})
</script>

<template>
  <div class="badge-page">
    <div class="badge-card">
      <!-- Live indicator pill (top-right) -->
      <div class="live-indicator" :title="natsStore.status">
        <span class="live-dot" :class="natsStatusDotClass"></span>
        <span class="live-label" :class="natsStatusDotClass">{{ natsStatusLabel }}</span>
      </div>

      <!-- Decorative header band -->
      <div class="badge-header">
        <div class="badge-header-pattern"></div>
      </div>

      <!-- Avatar with gradient ring -->
      <div class="badge-avatar">
        <div class="avatar-ring">
          <div v-if="getAvatarUrl()" class="avatar-img">
            <img :src="getAvatarUrl()!" alt="avatar" />
          </div>
          <div v-else class="avatar-placeholder">
            <span>{{ userName[0]?.toUpperCase() }}</span>
          </div>
        </div>
      </div>

      <!-- Identity info -->
      <div class="badge-body">
        <h1 class="badge-name">{{ userName }}</h1>
        <p class="badge-email">{{ userEmail }}</p>

        <div class="badge-tags">
          <span class="badge-tag badge-tag--org">{{ orgName }}</span>
          <span class="badge-tag badge-tag--role">{{ roleName }}</span>
        </div>

        <!-- Divider -->
        <div class="badge-divider"></div>

        <!-- Verification section: QR + clock/metadata -->
        <div v-if="hasNatsIdentity" class="badge-verification">
          <div class="badge-qr">
            <img
              v-if="qrDataUrl"
              :src="qrDataUrl"
              alt="Identity QR"
              class="badge-qr-img"
            />
            <div v-else class="badge-qr-placeholder">
              <span class="loading loading-spinner loading-sm"></span>
            </div>
          </div>

          <div class="badge-meta">
            <div class="badge-clock">
              <span class="badge-clock-time">{{ clockTime }}</span>
              <span class="badge-clock-date">{{ clockDate }}</span>
            </div>
            <div v-if="truncatedNKey" class="badge-nkey">
              {{ truncatedNKey }}
            </div>
            <div v-if="memberSince" class="badge-since">
              Since {{ memberSince }}
            </div>
          </div>
        </div>

        <!-- No NATS identity fallback -->
        <div v-else class="badge-no-identity">
          <div class="badge-clock badge-clock--centered">
            <span class="badge-clock-time">{{ clockTime }}</span>
            <span class="badge-clock-date">{{ clockDate }}</span>
          </div>
          <p class="badge-no-identity-text">No identity linked</p>
        </div>
      </div>

      <!-- Action -->
      <div class="badge-footer">
        <router-link :to="dashboardPath" class="badge-action-btn">
          Open Dashboard
        </router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ========================================
   Page Container
   ======================================== */
.badge-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 4rem);
  margin: -1rem;
  width: calc(100% + 2rem);
  padding: 1rem;
  background: oklch(var(--b2));
  box-sizing: border-box;
}

/* ========================================
   Card
   ======================================== */
.badge-card {
  position: relative;
  width: 100%;
  max-width: 420px;
  background: oklch(var(--b1));
  border: 1px solid oklch(var(--b3));
  border-radius: 1.5rem;
  overflow: hidden;
  box-shadow:
    0 2px 4px rgba(0, 0, 0, 0.04),
    0 8px 16px rgba(0, 0, 0, 0.08),
    0 20px 40px -8px rgba(0, 0, 0, 0.12);
}

/* ========================================
   Live Indicator Pill
   ======================================== */
.live-indicator {
  position: absolute;
  top: 0.875rem;
  right: 0.875rem;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.3rem 0.625rem 0.3rem 0.5rem;
  border-radius: 9999px;
  background: oklch(var(--b1) / 0.6);
  backdrop-filter: blur(6px);
}

.live-dot {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.live-label {
  font-size: 0.625rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  line-height: 1;
}

.live--connected .live-dot,
.live-dot.live--connected {
  background: oklch(var(--su));
  box-shadow: 0 0 6px 2px oklch(var(--su) / 0.5);
  animation: beacon 1.8s ease-in-out infinite;
}
.live-label.live--connected { color: oklch(var(--su)); }

.live--warning .live-dot,
.live-dot.live--warning {
  background: oklch(var(--wa));
  box-shadow: 0 0 6px 2px oklch(var(--wa) / 0.4);
  animation: beacon-warn 1s ease-in-out infinite;
}
.live-label.live--warning { color: oklch(var(--wa)); }

.live--error .live-dot,
.live-dot.live--error {
  background: oklch(var(--er));
  box-shadow: 0 0 4px 1px oklch(var(--er) / 0.3);
}
.live-label.live--error { color: oklch(var(--er)); }

@keyframes beacon {
  0%, 100% {
    box-shadow: 0 0 4px 1px oklch(var(--su) / 0.4);
    transform: scale(1);
  }
  50% {
    box-shadow: 0 0 12px 6px oklch(var(--su) / 0.35), 0 0 24px 12px oklch(var(--su) / 0.1);
    transform: scale(1.15);
  }
}

@keyframes beacon-warn {
  0%, 100% {
    box-shadow: 0 0 4px 1px oklch(var(--wa) / 0.4);
    transform: scale(1);
  }
  50% {
    box-shadow: 0 0 10px 5px oklch(var(--wa) / 0.4), 0 0 20px 10px oklch(var(--wa) / 0.15);
    transform: scale(1.15);
  }
}

/* ========================================
   Header Band
   ======================================== */
.badge-header {
  height: 120px;
  background: linear-gradient(135deg, oklch(var(--p)), oklch(var(--p) / 0.7));
  position: relative;
  overflow: hidden;
}

.badge-header-pattern {
  position: absolute;
  inset: 0;
  opacity: 0.08;
  background-image:
    radial-gradient(circle at 20% 50%, oklch(var(--pc)) 1px, transparent 1px),
    radial-gradient(circle at 80% 30%, oklch(var(--pc)) 1px, transparent 1px),
    radial-gradient(circle at 50% 80%, oklch(var(--pc)) 1px, transparent 1px);
  background-size: 40px 40px, 60px 60px, 50px 50px;
}

/* ========================================
   Avatar with Ring
   ======================================== */
.badge-avatar {
  display: flex;
  justify-content: center;
  margin-top: -60px;
  position: relative;
  z-index: 5;
}

.avatar-ring {
  width: 128px;
  height: 128px;
  border-radius: 50%;
  padding: 3px;
  background: oklch(var(--p) / 0.3);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  flex-shrink: 0;
}

.avatar-img,
.avatar-placeholder {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  border: 3px solid oklch(var(--b1));
  overflow: hidden;
}

.avatar-img img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: oklch(var(--n));
  color: oklch(var(--nc));
  font-size: 2.75rem;
  font-weight: 700;
}

/* ========================================
   Body
   ======================================== */
.badge-body {
  padding: 1rem 1.75rem 0;
  text-align: center;
}

.badge-name {
  font-size: 1.625rem;
  font-weight: 800;
  letter-spacing: -0.025em;
  color: oklch(var(--bc));
  margin: 0.5rem 0 0.25rem;
}

.badge-email {
  font-size: 0.875rem;
  color: oklch(var(--bc) / 0.5);
  margin: 0;
}

.badge-tags {
  display: flex;
  justify-content: center;
  gap: 0.5rem;
  margin-top: 1rem;
}

.badge-tag {
  padding: 0.3rem 0.875rem;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.025em;
  text-transform: capitalize;
}

.badge-tag--org {
  background: oklch(var(--b2));
  color: oklch(var(--bc) / 0.7);
  border: 1px solid oklch(var(--b3));
}

.badge-tag--role {
  background: oklch(var(--p) / 0.1);
  color: oklch(var(--p));
  border: 1px solid oklch(var(--p) / 0.15);
}

/* ========================================
   Divider
   ======================================== */
.badge-divider {
  margin: 1.25rem 0;
  border-top: 1px dashed oklch(var(--bc) / 0.12);
}

/* ========================================
   Verification Section (QR + Metadata)
   ======================================== */
.badge-verification {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: oklch(var(--b2) / 0.5);
  border: 1px solid oklch(var(--b3) / 0.5);
  border-radius: 0.75rem;
  text-align: left;
}

.badge-qr {
  flex-shrink: 0;
}

.badge-qr-img {
  width: 100px;
  height: 100px;
  border-radius: 0.5rem;
  display: block;
}

.badge-qr-placeholder {
  width: 100px;
  height: 100px;
  border-radius: 0.5rem;
  background: oklch(var(--b2));
  border: 1px solid oklch(var(--b3));
  display: flex;
  align-items: center;
  justify-content: center;
}

.badge-meta {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}

/* ========================================
   Clock
   ======================================== */
.badge-clock {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.badge-clock--centered {
  align-items: center;
  padding: 1rem;
  background: oklch(var(--b2));
  border: 1px solid oklch(var(--b3));
  border-radius: 0.75rem;
}

.badge-clock-time {
  font-size: 1.125rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: oklch(var(--bc));
  letter-spacing: 0.025em;
  white-space: nowrap;
}

.badge-clock-date {
  font-size: 0.75rem;
  color: oklch(var(--bc) / 0.5);
  font-weight: 500;
}

/* ========================================
   NKey & Member Since
   ======================================== */
.badge-nkey {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: oklch(var(--bc) / 0.5);
  letter-spacing: 0.025em;
}

.badge-since {
  font-size: 0.75rem;
  color: oklch(var(--bc) / 0.4);
  font-weight: 500;
}

/* ========================================
   No Identity Fallback
   ======================================== */
.badge-no-identity {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
}

.badge-no-identity-text {
  font-size: 0.75rem;
  color: oklch(var(--bc) / 0.4);
  font-style: italic;
  margin: 0;
}

/* ========================================
   Footer
   ======================================== */
.badge-footer {
  padding: 1.25rem 1.75rem 1.5rem;
}

.badge-action-btn {
  display: block;
  width: 100%;
  padding: 0.875rem 1rem;
  background: oklch(var(--p));
  color: oklch(var(--pc));
  border: none;
  border-radius: 0.75rem;
  font-size: 1rem;
  font-weight: 600;
  text-align: center;
  text-decoration: none;
  cursor: pointer;
  transition: opacity 0.2s, transform 0.1s;
}

.badge-action-btn:hover {
  opacity: 0.9;
}

.badge-action-btn:active {
  transform: scale(0.98);
}

/* ========================================
   Responsive
   ======================================== */
@media (min-width: 640px) {
  .badge-card {
    max-width: 440px;
  }

  .badge-header {
    height: 140px;
  }

  .badge-avatar {
    margin-top: -70px;
  }

  .avatar-ring {
    width: 148px;
    height: 148px;
  }

  .avatar-placeholder {
    font-size: 3.5rem;
  }

  .badge-name {
    font-size: 1.875rem;
  }

  .badge-qr-img,
  .badge-qr-placeholder {
    width: 120px;
    height: 120px;
  }

  .badge-clock-time {
    font-size: 1.25rem;
  }
}

/* Desktop: AppHeader hidden, sidebar separate */
@media (min-width: 1024px) {
  .badge-page {
    min-height: 100vh;
    margin: -1.5rem;
    width: calc(100% + 3rem);
  }
}
</style>
