<!-- src/components/common/ErrorBoundary.vue -->
<template>
  <div v-if="error" class="error-boundary">
    <div class="error-container">
      <div class="error-icon">⚠️</div>
      <h2 class="error-title">Something went wrong</h2>
      <div class="error-message">{{ error.message }}</div>
      <details v-if="error.stack" class="error-details">
        <summary>Technical Details</summary>
        <pre>{{ error.stack }}</pre>
      </details>
      <div class="error-actions">
        <button class="btn-primary" @click="handleReload">
          Reload Application
        </button>
        <button class="btn-secondary" @click="handleReset">
          Reset & Clear Data
        </button>
      </div>
      <div class="error-hint">
        If this persists, check the browser console for more details.
      </div>
    </div>
  </div>
  <slot v-else />
</template>

<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'
import { useConfirm } from '@/composables/useConfirm'

const { confirm } = useConfirm()

/**
 * Error Boundary Component
 * 
 * Catches unhandled errors in child components and displays a friendly error UI.
 * Prevents the entire app from crashing due to a single component error.
 * 
 * Usage:
 * <ErrorBoundary>
 *   <YourComponent />
 * </ErrorBoundary>
 */

const error = ref<Error | null>(null)

// Capture errors from child components
onErrorCaptured((err, instance, info) => {
  console.error('[ErrorBoundary] Caught error:', err)
  console.error('[ErrorBoundary] Component:', instance)
  console.error('[ErrorBoundary] Error info:', info)
  
  error.value = err as Error
  
  // Stop propagation to prevent app crash
  return false
})

/**
 * Reload the application
 */
function handleReload() {
  window.location.reload()
}

/**
 * Reset application state and reload
 * Clears localStorage to give a fresh start
 */
async function handleReset() {
  const confirmed = await confirm({
    title: 'Reset Application',
    message: 'Are you sure you want to reset the application?',
    details: 'This will clear all saved dashboards and settings. You will need to reconfigure the application.',
    confirmText: 'Reset & Clear Data',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    localStorage.clear()
    window.location.reload()
  } catch (err) {
    console.error('Failed to clear storage:', err)
    // Fallback: just reload
    window.location.reload()
  }
}
</script>

<style scoped>
.error-boundary {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
  background: var(--bg, #0a0a0a);
}

.error-container {
  max-width: 600px;
  width: 100%;
  background: var(--panel, #161616);
  border: 1px solid var(--border, #333);
  border-radius: 8px;
  padding: 32px;
  text-align: center;
}

.error-icon {
  font-size: 64px;
  margin-bottom: 16px;
  animation: shake 0.5s ease-in-out;
}

@keyframes shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-10px); }
  75% { transform: translateX(10px); }
}

.error-title {
  margin: 0 0 16px 0;
  font-size: 24px;
  font-weight: 600;
  color: var(--danger, #f85149);
}

.error-message {
  margin-bottom: 24px;
  font-size: 16px;
  color: var(--text, #e0e0e0);
  line-height: 1.5;
}

.error-details {
  margin-bottom: 24px;
  text-align: left;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 4px;
  padding: 16px;
  cursor: pointer;
}

.error-details summary {
  color: var(--muted, #888);
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 12px;
}

.error-details[open] summary {
  margin-bottom: 12px;
}

.error-details pre {
  margin: 0;
  color: var(--danger, #f85149);
  font-size: 12px;
  font-family: var(--mono, monospace);
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.4;
}

.error-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  margin-bottom: 16px;
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.btn-primary {
  background: var(--primary, #3fb950);
  color: white;
}

.btn-primary:hover {
  background: var(--primary-hover, #2ea043);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text, #e0e0e0);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.15);
}

.error-hint {
  font-size: 13px;
  color: var(--muted, #888);
  line-height: 1.4;
}
</style>
