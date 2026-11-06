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
  
  let json = ''
  try {
    if (typeof props.data === 'string') {
      const trimmed = props.data.trim()
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

  return json.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?)|(\b(true|false|null)\b)|(-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let cls = 'json-number'
      
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          cls = 'json-key'
          const content = match.substring(0, match.length - 1)
          return `<span class="${cls}">${escapeHtml(content)}</span>:`
        } else {
          cls = 'json-string'
          return `<span class="${cls}">${escapeHtml(match)}</span>`
        }
      } 
      
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
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text);
}

.json-key {
  color: var(--color-primary);
  font-weight: 600;
}

.json-string {
  color: var(--text);
}

.json-number {
  color: var(--color-secondary);
}

.json-boolean {
  color: var(--color-error);
  font-weight: 600;
}

.json-null {
  color: var(--muted);
  font-style: italic;
}
</style>
