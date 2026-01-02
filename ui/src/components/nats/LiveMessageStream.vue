<script setup lang="ts">
import { ref, onUnmounted, watch, onMounted } from 'vue'
import { type Subscription } from '@nats-io/nats-core'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'

const natsStore = useNatsStore()
const toast = useToast()

// --- State ---
const isStreaming = ref(false)
const messages = ref<any[]>([])
const showSettings = ref(false)

// Config State (Persisted)
const configuredSubjects = ref<string[]>([])
const newSubjectInput = ref('')

// Active Subscriptions
const subscriptions = ref<Subscription[]>([])

// --- Persistence ---
const STORAGE_KEY = 'stone_age_stream_subjects'

function loadSettings() {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) {
    try {
      configuredSubjects.value = JSON.parse(saved)
    } catch {
      configuredSubjects.value = ['>']
    }
  } else {
    configuredSubjects.value = ['>']
  }
}

function saveSettings() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(configuredSubjects.value))
}

function addSubject() {
  const val = newSubjectInput.value.trim()
  if (!val) return
  
  if (configuredSubjects.value.includes(val)) {
    newSubjectInput.value = ''
    return
  }

  configuredSubjects.value.push(val)
  newSubjectInput.value = ''
  saveSettings()
}

function removeSubject(subject: string) {
  configuredSubjects.value = configuredSubjects.value.filter(s => s !== subject)
  // If list becomes empty, user probably wants '>' implicitly, but we'll leave it empty in UI
  // and handle the default in startStream logic if needed.
  saveSettings()
}

// --- Streaming Logic ---

function toggleStream() {
  if (isStreaming.value) {
    stopStream()
  } else {
    startStream()
  }
}

async function startStream() {
  if (!natsStore.nc || natsStore.nc.isClosed()) {
    toast.error('NATS is not connected. Check settings.')
    return
  }

  // Use configured subjects, or default to '>' if list is empty
  const subjectsToSub = configuredSubjects.value.length > 0 
    ? configuredSubjects.value 
    : ['>']

  try {
    isStreaming.value = true
    
    // Subscribe to ALL subjects
    for (const subject of subjectsToSub) {
      console.log(`Subscribing to ${subject}`)
      const sub = natsStore.nc.subscribe(subject)
      subscriptions.value.push(sub)
      
      // Launch async iterator for this specific subscription
      // Note: We don't await this, we let it run in background
      handleSubscription(sub)
    }

  } catch (err: any) {
    toast.error(`Stream error: ${err.message}`)
    stopStream()
  }
}

async function handleSubscription(sub: Subscription) {
  try {
    for await (const m of sub) {
      // Decode
      let payload: any = ''
      try {
        try { payload = m.json() } catch { payload = m.string() }
      } catch {
        payload = '[Binary/Raw Data]'
      }

      const entry = {
        id: crypto.randomUUID(),
        subject: m.subject,
        payload: payload,
        timestamp: new Date()
      }

      // Prepend to log
      messages.value.unshift(entry)

      // Buffer Limit
      if (messages.value.length > 50) {
        messages.value.pop()
      }
    }
  } catch (err) {
    // Subscription closed or errored
    console.debug('Subscription closed', err)
  }
}

function stopStream() {
  // Unsubscribe all
  for (const sub of subscriptions.value) {
    sub.unsubscribe()
  }
  subscriptions.value = []
  isStreaming.value = false
}

function clearMessages() {
  messages.value = []
}

// Watch connection status to auto-stop
watch(() => natsStore.status, (newStatus) => {
  if (newStatus !== 'connected' && isStreaming.value) {
    stopStream()
  }
})

// Lifecycle
onMounted(() => {
  loadSettings()
})

onUnmounted(() => {
  stopStream()
})
</script>

<template>
  <div class="card bg-base-100 shadow-xl h-[500px] flex flex-col">
    <!-- Header -->
    <div class="card-body pb-2 flex-none">
      <div class="flex justify-between items-center">
        <div>
          <h2 class="card-title flex items-center gap-2">
            <span>📡 Live Event Bus</span>
            <div class="badge" :class="isStreaming ? 'badge-success animate-pulse' : 'badge-ghost'">
              {{ isStreaming ? 'LIVE' : 'PAUSED' }}
            </div>
          </h2>
          <div class="text-xs opacity-60 mt-1 flex flex-wrap gap-1 items-center">
            <span>Subscribed to:</span>
            <span v-if="configuredSubjects.length === 0" class="badge badge-xs badge-ghost">All (&gt;)</span>
            <span v-else v-for="s in configuredSubjects" :key="s" class="badge badge-xs badge-neutral font-mono">
              {{ s }}
            </span>
          </div>
        </div>
        
        <div class="flex gap-2">
          <button v-if="!isStreaming" @click="showSettings = true" class="btn btn-sm btn-ghost">
            ⚙️ Configure
          </button>
          <button v-if="messages.length > 0" @click="clearMessages" class="btn btn-sm btn-ghost" title="Clear Log">
            🚫
          </button>
          <button 
            @click="toggleStream" 
            class="btn btn-sm w-24"
            :class="isStreaming ? 'btn-error btn-outline' : 'btn-primary'"
          >
            {{ isStreaming ? 'Stop' : 'Start' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Terminal / Log Area -->
    <div class="flex-1 overflow-y-auto p-4 bg-base-200/50 border-t border-b border-base-200 font-mono text-xs space-y-2">
      
      <div v-if="messages.length === 0" class="h-full flex flex-col items-center justify-center opacity-40">
        <span class="text-4xl mb-2">📥</span>
        <p>Waiting for messages...</p>
        <p class="text-[10px]" v-if="!isStreaming">Click Start to connect</p>
      </div>

      <div 
        v-for="msg in messages" 
        :key="msg.id" 
        class="bg-base-100 p-3 rounded border-l-4 border-primary shadow-sm hover:shadow-md transition-shadow"
      >
        <div class="flex justify-between items-start mb-1 opacity-70">
          <span class="font-bold text-primary break-all">{{ msg.subject }}</span>
          <span class="text-[10px] whitespace-nowrap ml-2">{{ formatDate(msg.timestamp, 'HH:mm:ss.SSS') }}</span>
        </div>
        <div class="overflow-x-auto text-base-content/80">
          <pre v-if="typeof msg.payload === 'object'">{{ JSON.stringify(msg.payload, null, 2) }}</pre>
          <span v-else class="break-all whitespace-pre-wrap">{{ msg.payload }}</span>
        </div>
      </div>

    </div>
    
    <!-- Footer / Connection Warning -->
    <div v-if="!natsStore.isConnected" class="bg-warning/10 p-2 text-center text-xs text-warning border-t border-warning/20">
      ⚠️ NATS Disconnected. Check <router-link to="/settings" class="link">Settings</router-link>.
    </div>
  </div>

  <!-- Configuration Modal -->
  <dialog class="modal" :class="{ 'modal-open': showSettings }">
    <div class="modal-box">
      <h3 class="font-bold text-lg">Stream Configuration</h3>
      <p class="py-4 text-sm opacity-70">
        Add subjects to filter the live stream. Supports wildcards (<code>*</code>, <code>&gt;</code>).
      </p>
      
      <div class="form-control">
        <label class="label">
          <span class="label-text">Add Subject</span>
        </label>
        <div class="join">
          <input 
            v-model="newSubjectInput" 
            @keyup.enter="addSubject"
            type="text" 
            class="input input-bordered font-mono w-full join-item" 
            placeholder="e.g. location.ny.>" 
          />
          <button @click="addSubject" class="btn btn-primary join-item">Add</button>
        </div>
      </div>

      <div class="mt-4">
        <label class="label"><span class="label-text text-xs uppercase font-bold opacity-50">Active Filters</span></label>
        
        <div v-if="configuredSubjects.length === 0" class="alert bg-base-200 text-xs py-2">
          <span>No filters set. Defaulting to <strong>&gt;</strong> (All messages).</span>
        </div>

        <div class="flex flex-wrap gap-2">
          <div v-for="sub in configuredSubjects" :key="sub" class="badge badge-lg gap-2 pr-1">
            <span class="font-mono text-xs">{{ sub }}</span>
            <button @click="removeSubject(sub)" class="btn btn-ghost btn-xs btn-circle h-5 w-5 min-h-0 text-error">✕</button>
          </div>
        </div>
      </div>

      <div class="modal-action">
        <button class="btn" @click="showSettings = false">Done</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="showSettings = false"><button>close</button></form>
  </dialog>
</template>
