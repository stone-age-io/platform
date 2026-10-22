// ui/src/components/dashboard/widgets.ts
//
// Every static fact about a widget type, in one object.
//
// WHAT THIS REPLACED. Adding a widget used to mean editing six places in three
// directories, two of which failed in silence: a 16-case `switch` in
// WidgetContainer.vue, a `Record<WidgetType, Component>` of config panels in
// useWidgetForm.ts, two `WidgetType[]` arrays deciding which data-source panel
// a widget gets, two hardcoded `Set`s of strings deciding how it behaves on a
// phone, and a hand-written <button> in AddWidgetModal.vue. Forget the Sets and
// nothing errors — the widget just behaves wrong on a phone. Forget the palette
// entry and the widget cannot be added at all.
//
// THIS IS A MAP, NOT A PLUGIN SYSTEM. There is no `import.meta.glob`, no
// manifest file, no self-registration and no config-schema-driven form
// generation. Those would be a framework to maintain forever for sixteen widget
// types that change about twice a year. An object literal is less code than
// what it replaced, it reads top to bottom, and "where does this widget come
// from" is answered by reading it rather than by understanding a mechanism.
//
// THE TYPE IS THE CHECKLIST. `Record<WidgetType, WidgetDefinition>` means a
// type added to WIDGET_TYPES with no entry here is a compile error, and a
// required field left off an entry is a compile error. That is the whole point:
// every one of the old six touchpoints is now either this file or a compile
// error pointing at it.
//
// ORDER IS LOAD-BEARING. The Add Widget palette renders these in declaration
// order, grouped by `category` in WIDGET_CATEGORIES order.

import { defineAsyncComponent, type Component } from 'vue'
import type { WidgetType } from '@/types/dashboard'

// Light widgets, imported eagerly — they are small and a dashboard usually has
// several. The heavy ones below are deferred so a dashboard without a chart
// never downloads ECharts.
import TextWidget from '@/components/widgets/TextWidget.vue'
import ButtonWidget from '@/components/widgets/ButtonWidget.vue'
import SwitchWidget from '@/components/widgets/SwitchWidget.vue'
import SliderWidget from '@/components/widgets/SliderWidget.vue'
import StatCardWidget from '@/components/widgets/StatCardWidget.vue'
import StatusWidget from '@/components/widgets/StatusWidget.vue'

const ChartWidget = defineAsyncComponent(() => import('@/components/widgets/ChartWidget.vue'))
const GaugeWidget = defineAsyncComponent(() => import('@/components/widgets/GaugeWidget.vue'))
const MapWidget = defineAsyncComponent(() => import('@/components/widgets/MapWidget.vue'))
const KvWidget = defineAsyncComponent(() => import('@/components/widgets/KvWidget.vue'))
const ConsoleWidget = defineAsyncComponent(() => import('@/components/widgets/ConsoleWidget.vue'))
const PublisherWidget = defineAsyncComponent(() => import('@/components/widgets/PublisherWidget.vue'))
const MarkdownWidget = defineAsyncComponent(() => import('@/components/widgets/MarkdownWidget.vue'))
const KvTableWidget = defineAsyncComponent(() => import('@/components/widgets/KvTableWidget.vue'))
const StreamTableWidget = defineAsyncComponent(() => import('@/components/widgets/StreamTableWidget.vue'))
const ScannerWidget = defineAsyncComponent(() => import('@/components/widgets/ScannerWidget.vue'))

// Config panels are ALL deferred. They are reachable only by opening the
// configure modal, and eager imports here would otherwise drag twenty editor
// components (ConfigMap alone pulls the marker editors) into the chunk that
// renders a dashboard nobody is editing.
const ConfigText = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigText.vue'))
const ConfigChart = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigChart.vue'))
const ConfigStat = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigStat.vue'))
const ConfigGauge = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigGauge.vue'))
const ConfigButton = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigButton.vue'))
const ConfigKv = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigKv.vue'))
const ConfigSwitch = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigSwitch.vue'))
const ConfigSlider = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigSlider.vue'))
const ConfigMap = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigMap.vue'))
const ConfigConsole = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigConsole.vue'))
const ConfigPublisher = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigPublisher.vue'))
const ConfigStatus = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigStatus.vue'))
const ConfigMarkdown = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigMarkdown.vue'))
const ConfigKvTable = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigKvTable.vue'))
const ConfigStreamTable = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigStreamTable.vue'))
const ConfigScanner = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigScanner.vue'))

/** Palette groupings, rendered in this order. */
export const WIDGET_CATEGORIES = ['Data Display', 'Controls', 'Layout', 'Tools'] as const
export type WidgetCategory = (typeof WIDGET_CATEGORIES)[number]

export interface WidgetDefinition {
  /** Rendered on the dashboard. */
  component: Component
  /** The panel in the configure modal. Deferred; see above. */
  config: Component

  // --- Add Widget palette ---
  label: string
  icon: string
  description: string
  category: WidgetCategory

  // --- Locked mobile dashboards ---
  // A locked mobile dashboard hides widget headers to save vertical space. The
  // two flags below are the exceptions, and both used to be a Set of strings
  // nothing checked.
  /** Keep the header (and so the title) on a locked mobile dashboard. */
  mobileTitled: boolean
  /** Offer a full-screen button there too — for widgets too dense to read inline. */
  mobileExpandable: boolean

  // --- Configure modal ---
  /**
   * Show the shared subject / JSONPath / buffer panel.
   *
   * False means the widget either publishes rather than subscribes (button,
   * switch, slider, publisher), or draws its own data-source UI because its
   * source is a KV bucket or a prefix (kv, kvtable, map, status, markdown,
   * scanner).
   */
  standardDataSource: boolean
  /** Take a list of subjects in that panel rather than one. */
  multiSubject: boolean
}

export const WIDGETS: Record<WidgetType, WidgetDefinition> = {
  // --- Data Display ---
  text: {
    component: TextWidget, config: ConfigText,
    label: 'Text', icon: '📝', description: 'Display latest value', category: 'Data Display',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: true, multiSubject: false,
  },
  stat: {
    component: StatCardWidget, config: ConfigStat,
    label: 'Stat Card', icon: '📊', description: 'KPI with trend', category: 'Data Display',
    mobileTitled: true, mobileExpandable: false,
    standardDataSource: true, multiSubject: false,
  },
  gauge: {
    component: GaugeWidget, config: ConfigGauge,
    label: 'Gauge', icon: '⏲️', description: 'Circular meter', category: 'Data Display',
    mobileTitled: true, mobileExpandable: false,
    standardDataSource: true, multiSubject: false,
  },
  status: {
    component: StatusWidget, config: ConfigStatus,
    label: 'Status', icon: '🚦', description: 'State mapping & watchdog', category: 'Data Display',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: false, multiSubject: false,
  },
  chart: {
    component: ChartWidget, config: ConfigChart,
    label: 'Chart', icon: '📈', description: 'Line chart over time', category: 'Data Display',
    mobileTitled: true, mobileExpandable: false,
    standardDataSource: true, multiSubject: true,
  },
  kv: {
    component: KvWidget, config: ConfigKv,
    label: 'KV Value', icon: '🗄️', description: 'Single KV entry', category: 'Data Display',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: false, multiSubject: false,
  },
  kvtable: {
    component: KvTableWidget, config: ConfigKvTable,
    label: 'KV Table', icon: '📋', description: 'Live KV bucket table', category: 'Data Display',
    mobileTitled: true, mobileExpandable: true,
    standardDataSource: false, multiSubject: false,
  },
  streamtable: {
    component: StreamTableWidget, config: ConfigStreamTable,
    label: 'Stream Table', icon: '📡', description: 'Live message stream as a table', category: 'Data Display',
    mobileTitled: true, mobileExpandable: true,
    standardDataSource: true, multiSubject: true,
  },

  // --- Controls ---
  button: {
    component: ButtonWidget, config: ConfigButton,
    label: 'Button', icon: '📤', description: 'Publish messages', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: false, multiSubject: false,
  },
  switch: {
    component: SwitchWidget, config: ConfigSwitch,
    label: 'Switch', icon: '🔄', description: 'Toggle control', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: false, multiSubject: false,
  },
  slider: {
    component: SliderWidget, config: ConfigSlider,
    label: 'Slider', icon: '🎚️', description: 'Range control', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: false, multiSubject: false,
  },
  publisher: {
    component: PublisherWidget, config: ConfigPublisher,
    label: 'Publisher', icon: '📨', description: 'Send ad-hoc messages', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: false, multiSubject: false,
  },

  // --- Layout ---
  markdown: {
    component: MarkdownWidget, config: ConfigMarkdown,
    label: 'Markdown', icon: '📝', description: 'Static text & images', category: 'Layout',
    mobileTitled: true, mobileExpandable: true,
    standardDataSource: false, multiSubject: false,
  },
  map: {
    component: MapWidget, config: ConfigMap,
    label: 'Map', icon: '🗺️', description: 'Geographic location', category: 'Layout',
    mobileTitled: true, mobileExpandable: true,
    standardDataSource: false, multiSubject: false,
  },
  console: {
    component: ConsoleWidget, config: ConfigConsole,
    label: 'Console', icon: '📟', description: 'Live log stream', category: 'Layout',
    mobileTitled: false, mobileExpandable: false,
    standardDataSource: true, multiSubject: true,
  },

  // --- Tools ---
  scanner: {
    component: ScannerWidget, config: ConfigScanner,
    label: 'Scanner', icon: '📷', description: 'QR code lookup', category: 'Tools',
    mobileTitled: true, mobileExpandable: true,
    standardDataSource: false, multiSubject: false,
  },
}
