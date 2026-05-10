<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNatsStore } from '@/stores/nats'
import { pb } from '@/utils/pb'
import { format } from 'date-fns'
import QRCode from 'qrcode'
import { Kvm } from '@nats-io/kv'
import { decodeBytes } from '@/utils/encoding'
import type { BadgeRecord } from '@/types/dashboard'
import BrandLogo from '@/components/common/BrandLogo.vue'

const route = useRoute()
const authStore = useAuthStore()
const natsStore = useNatsStore()

const userName = computed(() => authStore.user?.name || 'User')

// Badge users at /badge get badge dashboard, others at /my-badge get main dashboard
const isBadgeRoute = computed(() => route.path.startsWith('/badge'))
const dashboardPath = computed(() => isBadgeRoute.value ? '/badge/dashboard' : '/')

// Whether the user has a NATS identity
const hasNatsIdentity = computed(() => !!authStore.currentNatsUser)

const avatarUrl = computed(() => {
  if (!authStore.user?.avatar) return null
  return pb.files.getURL(authStore.user, (authStore.user as any).avatar, {
    thumb: '200x200',
    token: pb.authStore.token
  })
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

// Badge KV record (lifecycle + arbitrary metadata)
const badgeRecord = ref<BadgeRecord | null>(null)

const expiresLabel = computed(() => {
  const exp = badgeRecord.value?.expires_at
  if (!exp) return null
  const d = new Date(exp)
  if (Number.isNaN(d.getTime())) return null
  return format(d, 'MMM d, yyyy')
})

const issuedLabel = computed(() => {
  const iss = badgeRecord.value?.issued_at
  if (!iss) return null
  const d = new Date(iss)
  if (Number.isNaN(d.getTime())) return null
  return format(d, 'MMM d, yyyy')
})

const badgeIsRevoked = computed(() => badgeRecord.value?.revoked === true)

const hasLifecycleChips = computed(() =>
  badgeIsRevoked.value || !!expiresLabel.value || !!issuedLabel.value
)

const metadataEntries = computed(() => {
  const meta = badgeRecord.value?.metadata || {}
  return Object.entries(meta).map(([key, value]) => ({ key, value: formatMetaValue(value) }))
})

function formatMetaValue(v: any): string {
  if (v === null || v === undefined) return '-'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

async function loadBadgeRecord() {
  const pk = authStore.currentNatsUser?.public_key
  if (!pk || !natsStore.nc) {
    badgeRecord.value = null
    return
  }
  try {
    const kvm = new Kvm(natsStore.nc)
    const kv = await kvm.open('badges')
    const entry = await kv.get(pk)
    if (!entry || !entry.value) {
      badgeRecord.value = null
      return
    }
    const str = decodeBytes(entry.value)
    try {
      badgeRecord.value = JSON.parse(str) as BadgeRecord
    } catch {
      badgeRecord.value = null
    }
  } catch {
    // Bucket missing or other error — render badge without lifecycle chip
    badgeRecord.value = null
  }
}

// Reload when connection status / nats_user changes
watch(
  () => [authStore.currentNatsUser?.public_key, natsStore.status],
  () => { loadBadgeRecord() },
  { immediate: true }
)

// Attributes drawer
const drawerOpen = ref(false)

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && drawerOpen.value) drawerOpen.value = false
}

onMounted(() => {
  clockInterval = setInterval(() => { now.value = new Date() }, 1000)
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  if (clockInterval) clearInterval(clockInterval)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="badge-page">
    <div class="badge-card" :class="{ 'is-revoked': badgeIsRevoked }">
      <!-- Header band -->
      <div class="badge-header">
        <div class="badge-header-logo">
          <BrandLogo :size="80" />
        </div>
        <div v-if="badgeIsRevoked" class="badge-header-revoked">REVOKED</div>
      </div>

      <!-- Avatar with gradient ring -->
      <div class="badge-avatar">
        <div class="avatar-ring">
          <div v-if="avatarUrl" class="avatar-img">
            <img :src="avatarUrl" alt="avatar" />
          </div>
          <div v-else class="avatar-placeholder">
            <span>{{ userName[0]?.toUpperCase() }}</span>
          </div>
        </div>
      </div>

      <!-- Identity info -->
      <div class="badge-body">
        <h1 class="badge-name">{{ userName }}</h1>

        <!-- Divider -->
        <div class="badge-divider"></div>

        <template v-if="hasNatsIdentity">
          <!-- Verification: QR + live clock side-by-side -->
          <div class="badge-verification">
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
            <div class="badge-clock">
              <span class="badge-clock-time">{{ clockTime }}</span>
              <span class="badge-clock-date">{{ clockDate }}</span>
            </div>
          </div>

          <!-- Lifecycle chips + attributes trigger from `badges` KV, if present -->
          <div v-if="badgeRecord" class="badge-record-section">
            <div v-if="hasLifecycleChips" class="badge-lifecycle">
              <span v-if="badgeIsRevoked" class="lifecycle-chip lifecycle-chip--revoked">
                Revoked
              </span>
              <span v-else-if="expiresLabel" class="lifecycle-chip">
                Expires {{ expiresLabel }}
              </span>
              <span v-if="issuedLabel" class="lifecycle-chip">
                Issued {{ issuedLabel }}
              </span>
            </div>

            <button
              v-if="metadataEntries.length > 0"
              type="button"
              class="badge-attrs-trigger"
              @click="drawerOpen = true"
            >
              <span>View attributes</span>
              <span class="badge-attrs-count">{{ metadataEntries.length }}</span>
            </button>
          </div>
        </template>

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

      <!-- Attributes drawer (overlays the card) -->
      <Transition name="drawer">
        <div
          v-if="drawerOpen"
          class="badge-drawer"
          role="dialog"
          aria-modal="true"
          aria-labelledby="badge-drawer-title"
        >
          <div class="badge-drawer-header">
            <h2 id="badge-drawer-title" class="badge-drawer-title">Attributes</h2>
            <button
              type="button"
              class="badge-drawer-close"
              aria-label="Close attributes"
              @click="drawerOpen = false"
            >
              ✕
            </button>
          </div>
          <div class="badge-drawer-body">
            <dl class="attr-list">
              <div
                v-for="entry in metadataEntries"
                :key="entry.key"
                class="attr-row"
              >
                <dt class="attr-key">{{ entry.key }}</dt>
                <dd class="attr-value">{{ entry.value }}</dd>
              </div>
            </dl>
          </div>
        </div>
      </Transition>
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
  border-radius: 1.25rem;
  overflow: hidden;
  box-shadow: 0 4px 24px -8px oklch(0% 0 0 / 0.15);
}

/* ========================================
   Header Band
   ======================================== */
.badge-header {
  height: 96px;
  background: oklch(var(--p));
  position: relative;
}

.badge-header-logo {
  position: absolute;
  top: 1rem;
  left: 1rem;
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: oklch(var(--pc));
}

/* ========================================
   Avatar
   ======================================== */
.badge-avatar {
  display: flex;
  justify-content: center;
  margin-top: -56px;
  position: relative;
  z-index: 5;
}

.avatar-ring {
  width: 120px;
  height: 120px;
  border-radius: 50%;
  padding: 4px;
  background: oklch(var(--b1));
  box-shadow: 0 2px 8px oklch(0% 0 0 / 0.1);
  flex-shrink: 0;
}

.avatar-img,
.avatar-placeholder {
  width: 100%;
  height: 100%;
  border-radius: 50%;
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
  background: oklch(var(--p));
  color: oklch(var(--pc));
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

/* ========================================
   Divider
   ======================================== */
.badge-divider {
  margin: 0.75rem 0;
  border-top: 1px dashed oklch(var(--bc) / 0.12);
}

/* ========================================
   Verification Section (QR + clock, row)
   ======================================== */
.badge-verification {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem;
  background: oklch(var(--b2) / 0.5);
  border: 1px solid oklch(var(--b3) / 0.5);
  border-radius: 0.75rem;
}

.badge-qr {
  flex-shrink: 0;
}

.badge-qr-img,
.badge-qr-placeholder {
  width: 130px;
  height: 130px;
  border-radius: 0.5rem;
  display: block;
}

.badge-qr-placeholder {
  background: oklch(var(--b2));
  border: 1px solid oklch(var(--b3));
  display: flex;
  align-items: center;
  justify-content: center;
}

/* ========================================
   Clock
   ======================================== */
.badge-clock {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.125rem;
  min-width: 0;
}

.badge-clock--centered {
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
   Lifecycle Chips (from KV `badges`)
   ======================================== */
.badge-lifecycle {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  margin-top: 0.75rem;
  justify-content: center;
}

.lifecycle-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.2rem 0.625rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 600;
  background: oklch(var(--b2));
  color: oklch(var(--bc) / 0.7);
  border: 1px solid oklch(var(--b3));
}

.lifecycle-chip--revoked {
  background: oklch(var(--er) / 0.1);
  color: oklch(var(--er));
  border-color: oklch(var(--er) / 0.25);
}

/* ========================================
   Revoked State (loud, glance-readable)
   ======================================== */
.badge-card.is-revoked {
  border-color: oklch(var(--er));
  border-width: 2px;
}

.badge-card.is-revoked .badge-header {
  background: oklch(var(--er));
}

.badge-card.is-revoked .badge-header-logo {
  color: oklch(var(--erc));
}

.badge-header-revoked {
  position: absolute;
  top: 50%;
  right: 1.25rem;
  transform: translateY(-50%);
  font-size: 1.5rem;
  font-weight: 900;
  letter-spacing: 0.18em;
  color: oklch(var(--erc));
  text-shadow: 0 1px 2px oklch(0% 0 0 / 0.2);
}

.badge-card.is-revoked .avatar-img img {
  filter: grayscale(1);
  opacity: 0.7;
}

.badge-card.is-revoked .avatar-placeholder {
  background: oklch(var(--n));
  color: oklch(var(--nc));
}

/* ========================================
   Attributes Trigger
   ======================================== */
.badge-attrs-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0.75rem auto 0;
  padding: 0.4rem 0.875rem;
  border-radius: 9999px;
  background: transparent;
  color: oklch(var(--bc) / 0.7);
  border: 1px solid oklch(var(--b3));
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}

.badge-attrs-trigger:hover {
  background: oklch(var(--b2));
  color: oklch(var(--bc));
  border-color: oklch(var(--bc) / 0.3);
}

.badge-attrs-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.25rem;
  height: 1.25rem;
  padding: 0 0.375rem;
  border-radius: 9999px;
  background: oklch(var(--p) / 0.15);
  color: oklch(var(--p));
  font-size: 0.6875rem;
  font-weight: 700;
}

.badge-record-section {
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* ========================================
   Attributes Drawer (overlays badge-card)
   ======================================== */
.badge-drawer {
  position: absolute;
  inset: 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  background: oklch(var(--b1));
}

.badge-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid oklch(var(--b3));
  flex-shrink: 0;
}

.badge-drawer-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: oklch(var(--bc));
}

.badge-drawer-close {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: none;
  background: oklch(var(--b2));
  color: oklch(var(--bc) / 0.7);
  cursor: pointer;
  font-size: 0.875rem;
  line-height: 1;
  transition: background 0.15s, color 0.15s;
}

.badge-drawer-close:hover {
  background: oklch(var(--b3));
  color: oklch(var(--bc));
}

.badge-drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 0.25rem 0;
}

.attr-list {
  margin: 0;
  padding: 0;
}

.attr-row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.75rem 1.25rem;
  border-bottom: 1px solid oklch(var(--b2));
}

.attr-row:last-child {
  border-bottom: none;
}

.attr-key {
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: oklch(var(--bc) / 0.5);
}

.attr-value {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.875rem;
  color: oklch(var(--bc));
  word-break: break-word;
}

.drawer-enter-active,
.drawer-leave-active {
  transition: transform 0.25s ease;
}

.drawer-enter-from,
.drawer-leave-to {
  transform: translateY(100%);
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
  padding: 1rem 1.75rem 1.25rem;
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
    height: 116px;
  }

  .badge-avatar {
    margin-top: -68px;
  }

  .avatar-ring {
    width: 140px;
    height: 140px;
  }

  .avatar-placeholder {
    font-size: 3.25rem;
  }

  .badge-name {
    font-size: 1.875rem;
  }

  .badge-qr-img,
  .badge-qr-placeholder {
    width: 150px;
    height: 150px;
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
