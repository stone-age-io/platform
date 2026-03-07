<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNatsStore } from '@/stores/nats'
import { pb } from '@/utils/pb'
import { format } from 'date-fns'

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

function getAvatarUrl() {
  if (!authStore.user?.avatar) return null
  return pb.files.getUrl(authStore.user, (authStore.user as any).avatar, {
    thumb: '200x200',
    token: pb.authStore.token
  })
}

const natsStatusDotClass = computed(() => {
  switch (natsStore.status) {
    case 'connected': return 'live-dot--connected'
    case 'connecting':
    case 'reconnecting': return 'live-dot--warning'
    default: return 'live-dot--error'
  }
})

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
      <!-- Live indicator (top-right) -->
      <div class="live-indicator" :title="natsStore.status">
        <span class="live-dot" :class="natsStatusDotClass"></span>
      </div>

      <!-- Decorative header band -->
      <div class="badge-header">
        <div class="badge-header-pattern"></div>
      </div>

      <!-- Avatar overlapping the header -->
      <div class="badge-avatar">
        <div v-if="getAvatarUrl()" class="avatar-img">
          <img :src="getAvatarUrl()!" alt="avatar" />
        </div>
        <div v-else class="avatar-placeholder">
          <span>{{ userName[0]?.toUpperCase() }}</span>
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

        <!-- Live clock -->
        <div class="badge-clock">
          <span class="badge-clock-time">{{ clockTime }}</span>
          <span class="badge-clock-date">{{ clockDate }}</span>
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
.badge-page {
  display: flex;
  align-items: center;
  justify-content: center;
  /* Mobile: subtract AppHeader height (4rem) */
  min-height: calc(100vh - 4rem);
  /* Break out of MainLayout <main> padding */
  margin: -1rem;
  width: calc(100% + 2rem);
  padding: 1rem;
  background: oklch(var(--b2));
  box-sizing: border-box;
}

.badge-card {
  position: relative;
  width: 100%;
  /* Mobile-first: fill most of the width */
  max-width: 420px;
  background: oklch(var(--b1));
  border: 1px solid oklch(var(--b3));
  border-radius: 1.5rem;
  overflow: hidden;
  box-shadow:
    0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 20px 40px -8px rgba(0, 0, 0, 0.15);
}

/* ========================================
   Live Indicator
   ======================================== */
.live-indicator {
  position: absolute;
  top: 1rem;
  right: 1rem;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: oklch(var(--b1) / 0.6);
  backdrop-filter: blur(4px);
}

.live-dot {
  display: block;
  width: 20px;
  height: 20px;
  border-radius: 50%;
}

.live-dot--connected {
  background: oklch(var(--su));
  box-shadow: 0 0 6px 2px oklch(var(--su) / 0.5);
  animation: beacon 1.8s ease-in-out infinite;
}

.live-dot--warning {
  background: oklch(var(--wa));
  box-shadow: 0 0 6px 2px oklch(var(--wa) / 0.4);
  animation: beacon-warn 1s ease-in-out infinite;
}

.live-dot--error {
  background: oklch(var(--er));
  box-shadow: 0 0 4px 1px oklch(var(--er) / 0.3);
}

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
   Header band
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
  opacity: 0.1;
  background-image:
    radial-gradient(circle at 20% 50%, oklch(var(--pc)) 1px, transparent 1px),
    radial-gradient(circle at 80% 30%, oklch(var(--pc)) 1px, transparent 1px),
    radial-gradient(circle at 50% 80%, oklch(var(--pc)) 1px, transparent 1px);
  background-size: 40px 40px, 60px 60px, 50px 50px;
}

/* ========================================
   Avatar
   ======================================== */
.badge-avatar {
  display: flex;
  justify-content: center;
  margin-top: -60px;
  position: relative;
  z-index: 5;
}

.avatar-img,
.avatar-placeholder {
  width: 120px;
  height: 120px;
  border-radius: 50%;
  border: 4px solid oklch(var(--b1));
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  flex-shrink: 0;
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
  padding: 1.25rem 2rem 0;
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
  margin-top: 1.25rem;
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
  background: oklch(var(--p) / 0.15);
  color: oklch(var(--p));
}

/* ========================================
   Live Clock
   ======================================== */
.badge-clock {
  margin-top: 1.5rem;
  padding: 1rem;
  background: oklch(var(--b2));
  border: 1px solid oklch(var(--b3));
  border-radius: 0.75rem;
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 0.75rem;
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
  font-size: 0.8125rem;
  color: oklch(var(--bc) / 0.5);
  font-weight: 500;
}

/* ========================================
   Footer
   ======================================== */
.badge-footer {
  padding: 1.5rem 2rem;
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

  .avatar-img,
  .avatar-placeholder {
    width: 140px;
    height: 140px;
  }

  .avatar-placeholder {
    font-size: 3.5rem;
  }

  .badge-name {
    font-size: 1.875rem;
  }

  .badge-clock-time {
    font-size: 1.25rem;
  }
}

/* Desktop: AppHeader is hidden, sidebar is separate — use full viewport */
@media (min-width: 1024px) {
  .badge-page {
    min-height: 100vh;
    margin: -1.5rem;
    width: calc(100% + 3rem);
  }
}
</style>
