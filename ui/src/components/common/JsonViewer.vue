<template>
  <pre class="json-viewer" v-html="highlightedHtml"></pre>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  data: any
}>()

const highlightedHtml = computed(() => {
  if (props.data === undefined) return ''
  
  // 1. Ensure we have a valid JSON string
  let json = ''
  try {
    if (typeof props.data === 'string') {
      const trimmed = props.data.trim()
      // If it looks like JSON object/array, format it. Otherwise treat as plain string.
      if ((trimmed.startsWith('{') || trimmed.startsWith('['))) {
        json = JSON.stringify(JSON.parse(props.data), null, 2)
      } else {
        return escapeHtml(props.data)
      }
    } else {
      json = JSON.stringify(props.data, null, 2)
    }
  } catch (e) {
    return escapeHtml(String(props.data))
  }

  // 2. Syntax Highlighting
  return json.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?)|(\b(true|false|null)\b)|(-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let cls = 'json-number'
      
      // Check if it starts with "
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          cls = 'json-key'
          // Separate the colon for cleaner coloring
          const content = match.substring(0, match.length - 1)
          return `<span class="${cls}">${escapeHtml(content)}</span>:`
        } else {
          cls = 'json-string'
          return `<span class="${cls}">${escapeHtml(match)}</span>`
        }
      } 
      
      // Booleans / Null
      else if (/true|false/.test(match)) {
        cls = 'json-boolean'
      } else if (/null/.test(match)) {
        cls = 'json-null'
      }
      
      return `<span class="${cls}">${escapeHtml(match)}</span>`
    }
  )
})

function escapeHtml(unsafe: string) {
  return unsafe
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;")
}
</script>

<style>
.json-viewer {
  margin: 0;
  font-family: var(--mono);
  font-size: 13px; /* Increased from 12px */
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word; /* Changed from break-all for better readability */
  color: var(--text);
}

.json-key {
  color: var(--color-primary); /* High contrast (Indigo/Purple) */
  font-weight: 600;
}

.json-string {
  color: var(--text); /* Standard text color for maximum readability */
}

.json-number {
  color: var(--color-secondary); /* Distinctive (Pink/Magenta) */
}

.json-boolean {
  color: var(--color-error); /* Red for boolean states */
  font-weight: 600;
}

.json-null {
  color: var(--muted);
  font-style: italic;
}
</style>
