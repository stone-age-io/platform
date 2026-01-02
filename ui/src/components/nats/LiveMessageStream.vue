<script setup lang="ts">
import { ref, onUnmounted, watch } from 'vue'
import { type Subscription } from '@nats-io/nats-core'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'

const natsStore = useNatsStore()
const toast = useToast()

// --- State ---
const isStreaming = ref(false)
const messages = ref<any[]>([])
const subjects = ref('>') // Default wildcard
const showSettings = ref(false)

// Subscription handle
let sub: Subscription | null = null

// --- Actions ---

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

  try {
    const subSubject = subjects.value.trim() || '>'
    console.log(`Subscribing to ${subSubject}`)
    
    sub = natsStore.nc.subscribe(subSubject)
    isStreaming.value = true
    
    // Async iterator for handling messages
    // This runs "forever" until sub.unsubscribe() or connection closes
    ;(async () => {
      if (!sub) return
      for await (const m of sub) {
        // Decode using new NATS v3 API (.string() / .json())
        let payload: any = ''
        try {
          // Attempt to parse as JSON directly
          try {
            payload = m.json()
          } catch {
            // Fallback to string if JSON parsing fails
            payload = m.string()
          }
        } catch {
          // Fallback if string decoding fails (binary data)
          payload = '[Binary/Raw Data]'
        }

        // Add to list
        const entry = {
          id: crypto.randomUUID(),
          subject: m.subject,
          payload: payload,
          timestamp: new Date()
        }

        messages.value.unshift(entry)

        // Grug Safety: Limit buffer to 50 items to keep DOM light
        if (messages.value.length > 50) {
          messages.value.pop()
        }
      }
    })().catch(err => {
      console.error('Stream error:', err)
      isStreaming.value = false
    })

  } catch (err: any) {
    toast.error(`Subscription failed: ${err.message}`)
    isStreaming.value = false
  }
}

function stopStream() {
  if (sub) {
    sub.unsubscribe()
    sub = null
  }
  isStreaming.value = false
}

function clearMessages() {
  messages.value = []
}

// Watch connection status - if we lose NATS, stop streaming UI
watch(() => natsStore.status, (newStatus) => {
  if (newStatus !== 'connected' && isStreaming.value) {
    stopStream()
  }
})

// Cleanup
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
          <p class="text-xs opacity-60 mt-1">
            Real-time messages from your infrastructure.
            <span v-if="isStreaming">Subscribed to: <code class="bg-base-200 px-1 rounded">{{ subjects }}</code></span>
          </p>
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
          <!-- Pretty Print JSON if object, else string -->
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
        Enter the NATS subjects to subscribe to. Supports wildcards (<code>*</code>, <code>&gt;</code>).
      </p>
      
      <div class="form-control">
        <label class="label">
          <span class="label-text">Subject Filter</span>
        </label>
        <input 
          v-model="subjects" 
          type="text" 
          class="input input-bordered font-mono" 
          placeholder="e.g. location.ny.>" 
        />
        <label class="label">
          <span class="label-text-alt">Default: <code>&gt;</code> (Listen to everything allowed by your role)</span>
        </label>
      </div>

      <div class="modal-action">
        <button class="btn" @click="showSettings = false">Done</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="showSettings = false"><button>close</button></form>
  </dialog>
</template>
