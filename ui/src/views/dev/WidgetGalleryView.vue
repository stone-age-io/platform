<!-- ui/src/views/dev/WidgetGalleryView.vue -->
<!--
  Every widget, rendered against fixtures, at /dev/widgets.

  WHY THIS EXISTS. 24,000 of the console's lines are the dashboard and its
  widgets, and no test renders any of them: `vue-tsc && vite build` stays green
  while a widget draws nothing. This is the cheapest thing that puts eyes on all
  sixteen at once — one page, no NATS, no login, no dashboard to set up.

  IT NEVER SHIPS. The route is registered inside `if (import.meta.env.DEV)`,
  which Vite replaces with `false` in a production build, so this component and
  its fixtures are dropped from the bundle rather than merely unreachable.

  NOT STORYBOOK, DELIBERATELY. A dependency, a second build and a second set of
  config files, to arrive at a page with sixteen widgets on it.
-->
<template>
  <div class="gallery" :data-theme="theme">
    <header class="gallery-header">
      <div>
        <h1>Widget gallery</h1>
        <p class="sub">
          {{ fixtures.length }} widget types, rendered from
          <code>src/views/dev/widgetFixtures.ts</code>. Dev builds only.
        </p>
      </div>
      <div class="controls">
        <label><input type="checkbox" v-model="mobile" /> Mobile layout</label>
        <label><input type="checkbox" v-model="dark" /> Dark</label>
        <button class="refill" @click="fill">Refill buffers</button>
      </div>
    </header>

    <p v-if="!natsStore.isConnected" class="offline-note">
      No NATS connection. Widgets that read KV or a stream directly show their
      empty state; everything fed by the message buffer renders as normal.
    </p>

    <div class="grid">
      <section v-for="f in fixtures" :key="f.widget.type" class="cell">
        <div class="cell-head">
          <span class="type">{{ f.widget.type }}</span>
          <span v-if="WIDGETS[f.widget.type].mobileTitled" class="flag" title="Keeps its header on a locked mobile dashboard">titled</span>
          <span v-if="WIDGETS[f.widget.type].mobileExpandable" class="flag" title="Offers a full-screen button on a locked mobile dashboard">expandable</span>
        </div>
        <p class="note">{{ f.note }}</p>
        <div class="frame" :style="{ height: f.widget.h * 80 + 'px' }">
          <WidgetContainer :config="f.widget" :is-mobile="mobile" />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import WidgetContainer from '@/components/dashboard/WidgetContainer.vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useNatsStore } from '@/stores/nats'
import { WIDGETS } from '@/components/dashboard/widgets'
import { buildFixtures } from './widgetFixtures'

const dataStore = useWidgetDataStore()
const natsStore = useNatsStore()

const mobile = ref(false)
const dark = ref(true)
const theme = computed(() => (dark.value ? 'dark' : 'light'))

const fixtures = ref(Object.values(buildFixtures()))

/**
 * Push each fixture's messages into the widget data buffer, which is where
 * every buffer-backed widget reads from. Timestamps are built at call time, so
 * refilling also resets anything that ages (trends, staleness, a chart window).
 */
function fill() {
  fixtures.value = Object.values(buildFixtures())
  for (const f of fixtures.value) {
    dataStore.clearBuffer(f.widget.id)
    dataStore.initializeBuffer(f.widget.id, f.widget.buffer.maxCount)
    if (f.messages.length === 0) continue
    dataStore.batchAddMessages(f.messages.map(m => ({ widgetId: f.widget.id, ...m })))
  }
}

onMounted(fill)
</script>

<style scoped>
.gallery {
  min-height: 100vh;
  background: oklch(var(--b2));
  color: oklch(var(--bc));
  padding: 24px;
}

.gallery-header {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
}

h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.sub {
  margin: 4px 0 0;
  font-size: 13px;
  color: oklch(var(--bc) / 0.6);
}

.sub code {
  font-size: 12px;
}

.controls {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 13px;
}

.controls label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.refill {
  background: oklch(var(--b3));
  color: oklch(var(--bc));
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  padding: 5px 12px;
  font-size: 13px;
  cursor: pointer;
}

.offline-note {
  margin: 12px 0 0;
  font-size: 13px;
  color: oklch(var(--bc) / 0.6);
  max-width: 70ch;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 24px;
  margin-top: 24px;
}

.cell-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.type {
  font-family: ui-monospace, monospace;
  font-size: 13px;
  font-weight: 600;
}

.flag {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 1px 6px;
  border-radius: 999px;
  background: oklch(var(--b3));
  color: oklch(var(--bc) / 0.7);
  cursor: help;
}

.note {
  margin: 4px 0 8px;
  font-size: 12px;
  line-height: 1.5;
  color: oklch(var(--bc) / 0.6);
}

.frame {
  min-height: 160px;
}
</style>
